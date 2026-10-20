document.addEventListener("DOMContentLoaded", function () {
  const map = L.map("map").setView([46.8, -110.0], 6);
  L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png").addTo(map);

  // Defining all constants that get used
  const highwayColors = { "I-90": "blue", "I-15": "red", "I-94": "green" };
  let isPlanning = false,
    startPoint = null,
    activeRoute = [];
  let routeLine = null,
    currentIndex = 0,
    allPointsGrouped = {};
  let allMarkers = [],
    activeWalkthroughMarker = null;
  let panelOpen = false,
    loginOpen = false,
    filterOpen = false;
  let weatherCache = {},
    autoplayInterval = null;
  let graph = {};
  const bottomPanel = document.getElementById("bottom-panel");
  const panelContent = document.getElementById("panel-content");
  const planBtn = document.getElementById("plan-btn");
  const arrow = document.getElementById("arrow");
  const popupLogin = document.getElementById("popupLogin");
  const openLoginBtn = document.getElementById("openLoginBtn");
  const popupFilter = document.getElementById("popupFilter");
  const openFilterBtn = document.getElementById("openFilterBtn");

  // ---------------------------------------------------- Grab the border points for montana and draw an outline around montana
  fetch("plot_points/border_points.php")
    .then((r) => r.json())
    .then((points) => {
      L.polygon(
        points.map((p) => [parseFloat(p.lat), parseFloat(p.lng)]),
        {
          color: "black",
          weight: 2,
          fill: false,
          interactive: false,
        },
      ).addTo(map);
    });

  // ---------------------------------------------------- Grab and plot all the highway points on montana,
  fetch("plot_points/highway_points.php")
    .then((r) => r.json())
    .then((grouped) => {
      allPointsGrouped = grouped;
      // object.keys to turn out object into an array, then loop through each arrary in that (highway), and loop
      // through each point
      Object.keys(grouped).forEach((highway) => {
        grouped[highway].forEach((p) => {
          // Make each point a marker on the map with coords, and fill with the right color
          const marker = L.circleMarker([p.lat, p.lng], {
            radius: 2,
            fillColor: highwayColors[highway],
            color: highwayColors[highway],
            weight: 0.5,
            fillOpacity: 0.8,
          }).addTo(map);
          // put a hover on the point to show the name if not N/a, else the coords
          marker.bindTooltip(
            p.name !== "N/A" ? p.name : `(${p.lat}, ${p.lng})`,
          );
          // bind the point data to the marker
          marker.pointData = p;
          // add this to our all Marker tracker so we odnt need to use DB
          allMarkers.push(marker);
          // atach a click listener to each point to eaither do route selection or solo weather
          marker.on("click", () =>
            isPlanning ? handleRouteSelection(p) : openWeatherForPoint(p),
          );
        });
      });
      // turn the highway points into a graph for routing
      buildGraph();
    });

  // ---------------------------------------------------- fetch and plot all the Points of interest w/ icon png
  fetch("plot_points/poi_points.php")
    .then((r) => r.json())
    .then((points) => {
      // define icons for the Points of Interest
      const icons = {
        city: L.icon({
          iconUrl: "images/city.png",
          iconSize: [28, 28],
          iconAnchor: [14, 14],
        }),
        pass: L.icon({
          iconUrl: "images/pass.png",
          iconSize: [28, 28],
          iconAnchor: [14, 14],
        }),
        sasquatch: L.icon({
          iconUrl: "images/sasquatch.png",
          iconSize: [28, 28],
          iconAnchor: [14, 14],
        }),
      };
      // attach the lat and long data to each marker and the correct icon, and add to the map
      points.forEach((p) => {
        const marker = L.marker([parseFloat(p.lat), parseFloat(p.lng)], {
          icon: icons[p.point_type],
        }).addTo(map);
        // same as above, attach the name, and coord to the point and then add a click listener
        marker.bindTooltip(p.name);
        marker.pointData = p;
        allMarkers.push(marker);
        marker.on("click", () =>
          isPlanning ? handleRouteSelection(p) : openWeatherForPoint(p),
        );
      });
      // Default to showing only highways not all the POI
      applyFilters();
    });

  // ---------------------------------------------------- Build the routing graph
  // Every point is a node, keyed by its coords so a junction that shows up on two
  // highways becomes one node. Points next to each other on a highway get an edge
  // weighted by the distance between them in miles.
  function buildGraph() {
    graph = {};
    for (const h in allPointsGrouped) {
      const pts = allPointsGrouped[h];
      pts.forEach((p) => {
        const k = keyOf(p);
        if (!graph[k]) graph[k] = { points: {}, edges: [] };
        // keep each highway's copy of the point so the route keeps the right highway name
        graph[k].points[h] = p;
      });
      for (let i = 1; i < pts.length; i++) {
        const a = keyOf(pts[i - 1]);
        const b = keyOf(pts[i]);
        const miles = distanceMiles(pts[i - 1], pts[i]);
        graph[a].edges.push({ to: b, miles, highway: h });
        graph[b].edges.push({ to: a, miles, highway: h });
      }
    }
  }

  const keyOf = (p) => `${p.lat},${p.lng}`;

  // ---------------------------------------------------- Distance between two points in miles (haversine formula)
  function distanceMiles(a, b) {
    const toRad = (d) => (d * Math.PI) / 180;
    const dLat = toRad(b.lat - a.lat);
    const dLng = toRad(b.lng - a.lng);
    const x =
      Math.sin(dLat / 2) ** 2 +
      Math.cos(toRad(a.lat)) * Math.cos(toRad(b.lat)) * Math.sin(dLng / 2) ** 2;
    return 3958.8 * 2 * Math.asin(Math.sqrt(x));
  }

  // ---------------------------------------------------- Map will only show the applied filter(s)
  function applyFilters() {
    // grab all the filter buttons on the filter form that are active, then loop through all the markers
    // and eaither add or remove from map acordingly
    const active = new Set(
      [...document.querySelectorAll(".filter-btn.active")].map(
        (b) => b.dataset.value,
      ),
    );
    allMarkers.forEach((m) =>
      active.has(m.pointData.point_type) ? m.addTo(map) : m.remove(),
    );
  }

  // ---------------------------------------------------- Used to clear all filters
  function hideAllMarkers() {
    allMarkers.forEach((m) => m.remove());
  }

  // ---------------------------------------------------- Routing
  function handleRouteSelection(clickedPoint) {
    // Route selection has to snap to the nearest highway point if not already a highway point
    const snapped = snapToNearest(clickedPoint);
    if (!startPoint) {
      startPoint = snapped;
      document.getElementById("status").innerHTML =
        `START SET: ${snapped.highway}. <span class="text-secondary">CLICK DESTINATION</span>`;
      return;
    }
    const route = findRoute(startPoint, snapped);
    activeRoute = route;
    isPlanning = false;

    // Log the search
    const first = route[0];
    const last = route[route.length - 1];

    fetch("log_route.php", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        start_lat: first.lat,
        start_lng: first.lng,
        start_highway: first.highway,
        end_lat: last.lat,
        end_lng: last.lng,
        end_highway: last.highway,
        route_length: route.length,
      }),
    });
    startWalkthrough();
  }

  // ----------------------------------------------------------------- Function to snap a POI to the nearest highway point
  function snapToNearest(point) {
    // if intermediate, junction or border point dont need a close highway point then
    if (
      point.point_type === "intermediate" ||
      point.point_type === "junction" ||
      point.point_type === "border"
    ) {
      return point;
    }

    // find the actual highway point object by id, in my data schema
    for (const h in allPointsGrouped) {
      const found = allPointsGrouped[h].find(
        (p) => p.id === point.nearest_highway_pt,
      );
      if (found) return found;
    }
    // Wont happen with how schema is set up, but if not found return original point
    return point;
  }

  //---------------------------------------------------------------------------------- Func to find a route between start and end
  // MOST COMPLEX FUNCTION in my opinion
  // Dijkstra's shortest path over the highway graph, so it works with any number of
  // junctions and always picks the shortest way instead of the first one it finds
  function findRoute(s, e) {
    const start = keyOf(s);
    const end = keyOf(e);
    if (!graph[start] || !graph[end]) return null;

    const dist = { [start]: 0 };
    // prev[node] = the node and highway we came from on the shortest path
    const prev = {};
    const visited = new Set();
    const queue = [start];

    while (queue.length) {
      // grab the closest unvisited node, the graph is small enough that a sort is fine
      queue.sort((a, b) => dist[a] - dist[b]);
      const cur = queue.shift();
      if (visited.has(cur)) continue;
      visited.add(cur);
      if (cur === end) break;

      for (const edge of graph[cur].edges) {
        const d = dist[cur] + edge.miles;
        if (dist[edge.to] === undefined || d < dist[edge.to]) {
          dist[edge.to] = d;
          prev[edge.to] = { from: cur, highway: edge.highway };
          queue.push(edge.to);
        }
      }
    }
    // wont happen with my set up, but no route found
    if (!visited.has(end)) return null;
    if (start === end) return [s];

    // walk back from the end to the start, using the point from the highway we drove in on
    const route = [];
    let k = end;
    while (k !== start) {
      route.unshift(graph[k].points[prev[k].highway]);
      k = prev[k].from;
    }
    // the start point uses the highway of the first stretch of road
    const firstHighway = prev[keyOf(route[0])].highway;
    route.unshift(graph[start].points[firstHighway]);
    return route;
  }

  // ---------------------------------------------------- Walk Through Functionality start
  function startWalkthrough() {
    // hides all other markers so route is distinct
    hideAllMarkers();
    // Define the start and end coordinates from your route
    const sPoint = activeRoute[0];
    const ePoint = activeRoute[activeRoute.length - 1];

    // make the next, prev and auot play button visuable
    document.getElementById("route-nav").classList.remove("d-none");
    // disable the play button while weather info loads
    document.getElementById("autoplay-btn").disabled = true;
    // draw the route line using polyline and points of route
    if (routeLine) map.removeLayer(routeLine);
    routeLine = L.polyline(
      activeRoute.map((pt) => [pt.lat, pt.lng]),
      { color: "blue", weight: 4, opacity: 0.5 },
    ).addTo(map);
    currentIndex = 0;
    updateWalkthroughStep();
    prefetchAllWeather(activeRoute);
  }

  // ---------------------------------------------------- Walk Through step funcitnality
  function updateWalkthroughStep() {
    const p = activeRoute[currentIndex];
    if (activeWalkthroughMarker) map.removeLayer(activeWalkthroughMarker);

    // Grab and Size my carIcon image
    const carIcon = L.icon({
      iconUrl: "images/car.png",
      iconSize: [40, 40],
      iconAnchor: [20, 20],
    });

    // Place the car image on the map
    activeWalkthroughMarker = L.marker([p.lat, p.lng], { icon: carIcon }).addTo(
      map,
    );

    // grab the route-index-text and update it
    document.getElementById("route-index-text").textContent =
      `POINT ${currentIndex + 1} OF ${activeRoute.length}`;
    // get the weather for each point
    openWeatherForPoint(p);
  }
  // ---------------------------------------------------- Reset Map Ui
  function resetMapUI(closePanel = true) {
    // stop autoplay if running
    stopAutoplay();
    // clr cache
    weatherCache = {};
    //remove car and route on map if it is there
    if (activeWalkthroughMarker) map.removeLayer(activeWalkthroughMarker);
    if (routeLine) map.removeLayer(routeLine);
    // restore old map markers
    applyFilters();
    // hide the route nav btns, and reset status text
    document.getElementById("route-nav").classList.add("d-none");
    document.getElementById("status").textContent = "Please Click a Point";
    document.getElementById("weather-content").innerHTML = "";
    planBtn.classList.remove("active");
    // reset routing states
    isPlanning = false;
    startPoint = null;
    activeRoute = [];
    // only close pannel if told to, else infinite loop
    if (closePanel) togglePanel(false);
  }

  // ---------------------------------------------------- Toggle bottom pannel
  function togglePanel(s = null) {
    // close both login and filter if open
    toggleLogin(false);
    toggleFilter(false);
    // use s if defined else flip the current state and use that
    panelOpen = s !== null ? s : !panelOpen;
    // change text and arrow on the panel and show / not show accordingly
    bottomPanel.classList.toggle("open", panelOpen);
    arrow.textContent = panelOpen ? "▼" : "▲";
    panelContent.classList.toggle("d-none", !panelOpen);

    // grab the route nav bar, and hide if panel is closed or the route length is zero
    document
      .getElementById("route-nav")
      .classList.toggle("d-none", !panelOpen || activeRoute.length === 0);

    // if the panel closed resetMapUI
    if (!panelOpen) {
      resetMapUI(false);
    }
  }
  // ---------------------------------------------------- used to toggle the Login Popup visibility
  function toggleLogin(s) {
    if (!popupLogin) return;
    // close the filer, and btm pannel if open
    if (s) toggleFilter(false);
    if (s) togglePanel(false);
    // redefine state
    loginOpen = s;
    // make btn active and show popup
    openLoginBtn.classList.toggle("active", s);
    popupLogin.classList.toggle("show", s);
  }

  // ---------------------------------------------------- used to toggle the Filter Popup visibility
  function toggleFilter(s) {
    if (!popupFilter) return;
    // close the login, and btm pannel if open
    if (s) toggleLogin(false);
    if (s) togglePanel(false);
    // redifne state
    filterOpen = s;
    // make the button be active and show the popup
    openFilterBtn.classList.toggle("active", s);
    popupFilter.classList.toggle("show", s);
  }

  // ---------------------------------------------------- Grab weather for point
  function openWeatherForPoint(point) {
    // auto open pannel if not already open
    if (!panelOpen) togglePanel(true);
    // if just invidual point selction, change the status text accordingly
    if (!isPlanning)
      document.getElementById("status").textContent =
        point.name !== "N/A" ? point.name : `(${point.lat}, ${point.lng})`;
    // if there is a cache of data use that to get the weather point
    const cached =
      activeRoute.length > 0 ? weatherCache[currentIndex] : undefined;
    if (cached !== undefined) {
      renderWeather(cached);
      return;
    }
    // else remove spinner, and fetch the weather using get_weather, parse response and call render Weather
    document.getElementById("weather-spinner").classList.remove("d-none");
    document.getElementById("weather-content").innerHTML = "";
    fetch(`get_weather.php?lat=${point.lat}&lng=${point.lng}`)
      .then((r) => r.json())
      .then(renderWeather);
  }

  // ---------------------------------------------------- Place weather data into a card
  function renderWeather(data) {
    // make spinner not visable
    document.getElementById("weather-spinner").classList.add("d-none");

    // check if the periods of data we got back is an array else use empty arrary
    const periods = Array.isArray(data) ? data : [];

    // if empty array call out weather Error function to display that
    if (!periods.length) {
      renderWeatherError("No forecast data available for this location.");
      return;
    }

    // else map through each period ( times of data ) and build a card to display it in
    document.getElementById("weather-content").innerHTML = periods
      .map(
        (per) => `
      <div class="card shadow-sm border-0">
        <div class="card-header bg-dark text-white py-1 small">${per.name}</div>
        <div class="card-body p-2 text-center">
          <div class="fw-bold fs-5">${per.temperature}°${per.temperatureUnit}</div>
          <div class="small text-muted" style="font-size:0.7rem">${per.shortForecast}</div>
        </div>
      </div>`,
      )
      .join("");
  }

  // ---------------------------------------------------- Style Error with featcing weather data in a card
  function renderWeatherError(message = "Error fetching weather data.") {
    // hide weather spinner  and construct the error card to show
    document.getElementById("weather-spinner").classList.add("d-none");
    document.getElementById("weather-content").innerHTML = `
    <div class="card shadow-sm border-0">
      <div class="card-header bg-danger text-white py-1 small">Weather Unavailable</div>
      <div class="card-body p-2 text-center">
        <div class="small text-muted">${message}</div>
      </div>
    </div>`;
  }

  // ---------------------------------------------------- Prefetch and store all weather data for walkthrough
  async function prefetchAllWeather(route) {
    // disable the arrow buttons at first so you cannot click while fetching weather
    document.getElementById("prev-btn").disabled = true;
    document.getElementById("next-btn").disabled = true;

    weatherCache = {};
    const bar = document.getElementById("prefetch-bar");
    const prog = document.getElementById("prefetch-progress");

    // makthe bar visable
    bar.classList.remove("d-none");
    prog.style.width = "0%";
    let done = 0;

    // promise to get ALL the weather data and store in the cache and increment the loader bar
    await Promise.all(
      route.map((point, i) =>
        fetch(`get_weather.php?lat=${point.lat}&lng=${point.lng}`)
          .then((r) => r.json())
          .catch(() =>
            renderWeatherError("Could not reach the weather service."),
          )
          .then((data) => {
            // save data
            weatherCache[i] = data;
            // adjust the width of the loader. done is how many routes we have got weather from, divide
            // by total length of route and multiply by 100 to get the percentage of completion
            prog.style.width = Math.round((++done / route.length) * 100) + "%";
          }),
      ),
    );

    // hide the loading bar once it is done
    bar.classList.add("d-none");

    // Activiate all the buttons to move through
    document.getElementById("autoplay-btn").disabled = false;
    document.getElementById("prev-btn").disabled = false;
    document.getElementById("next-btn").disabled = false;
  }

  // -------------------------------------------------- disable the buttons, and clear the intervalwhen clicked
  function stopAutoplay() {
    // clear interval, change auoplay-btn text and enable btns
    clearInterval(autoplayInterval);
    autoplayInterval = null;
    document.getElementById("autoplay-btn").textContent = "▶ Autoplay";
    document.getElementById("prev-btn").disabled = false;
    document.getElementById("next-btn").disabled = false;
  }

  function startAutoplay() {
    // if autoplay already running, cliking the btn again will stop it
    if (autoplayInterval) {
      stopAutoplay();
      return;
    }
    // disable btns
    document.getElementById("prev-btn").disabled = true;
    document.getElementById("next-btn").disabled = true;
    // change text on autoplay-btn
    const btn = document.getElementById("autoplay-btn");
    btn.textContent = "⏹ Stop";

    // interval is 2 seconds at each spot, and continue auto play as long as we are not at the end of the rt
    autoplayInterval = setInterval(() => {
      if (currentIndex < activeRoute.length - 1) {
        currentIndex++;
        updateWalkthroughStep();
      } else stopAutoplay();
    }, 2000);
  }

  // ---------------------------------------------------- EVENT Listeners

  // Event Lisnter to toggle panel open or close
  document
    .getElementById("pull-tab")
    .addEventListener("click", () => togglePanel());

  document.getElementById("plan-btn").addEventListener("click", () => {
    resetMapUI();
    isPlanning = true;
    planBtn.classList.add("active");
    document.getElementById("status").textContent = "SELECT START MARKER";
    if (!panelOpen) togglePanel(true);
  });

  // Event lisnter to start auto play functionality
  document
    .getElementById("autoplay-btn")
    .addEventListener("click", startAutoplay);

  // Event Listner pair to manually step through coord points
  document.getElementById("next-btn").addEventListener("click", () => {
    stopAutoplay();
    if (currentIndex < activeRoute.length - 1) {
      currentIndex++;
      updateWalkthroughStep();
    }
  });
  document.getElementById("prev-btn").addEventListener("click", () => {
    stopAutoplay();
    if (currentIndex > 0) {
      currentIndex--;
      updateWalkthroughStep();
    }
  });

  // Event listener pair to Open / close Login form
  document
    .getElementById("openLoginBtn")
    .addEventListener("click", () => toggleLogin(true));
  document
    .getElementById("closeLoginBtn")
    .addEventListener("click", () => toggleLogin(false));

  // Even listener pair to Open/Close Filter form and apply filters to map
  document
    .getElementById("openFilterBtn")
    .addEventListener("click", () => toggleFilter(true));
  document
    .getElementById("closeFilterBtn")
    .addEventListener("click", () => toggleFilter(false));
  document.querySelectorAll(".filter-btn").forEach((btn) =>
    btn.addEventListener("click", () => {
      btn.classList.toggle("active");
      applyFilters();
    }),
  );
});
