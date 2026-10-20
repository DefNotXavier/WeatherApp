document.addEventListener("DOMContentLoaded", function () {
  const map = L.map("map").setView([46.8, -110.0], 6);
  L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png").addTo(map);

  // Defining all constants that get used
  const highwayColors = { "I-90": "blue", "I-15": "red", "I-94": "green" };
  let isPlanning = false,
    startPoint = null,
    activeRoute = [],
    routeInfo = null;
  let routeLine = null,
    currentIndex = 0;
  let allMarkers = [],
    activeWalkthroughMarker = null;
  let panelOpen = false,
    loginOpen = false,
    filterOpen = false;
  let weatherCache = {},
    autoplayInterval = null;
  const bottomPanel = document.getElementById("bottom-panel");
  const panelContent = document.getElementById("panel-content");
  const planBtn = document.getElementById("plan-btn");
  const arrow = document.getElementById("arrow");
  const popupLogin = document.getElementById("popupLogin");
  const openLoginBtn = document.getElementById("openLoginBtn");
  const popupFilter = document.getElementById("popupFilter");
  const openFilterBtn = document.getElementById("openFilterBtn");

  // the login redirects back here with ?error=1 if the password was wrong
  if (new URLSearchParams(location.search).has("error"))
    document.getElementById("login-error").classList.remove("d-none");

  // ---------------------------------------------------- Draw the outline of montana
  // All the map data comes from the Go API as GeoJSON, straight out of PostGIS
  fetch("/api/border")
    .then((r) => r.json())
    .then((border) => {
      L.geoJSON(border, {
        style: { color: "black", weight: 2, fill: false },
        interactive: false,
      }).addTo(map);
    });

  // ---------------------------------------------------- Draw the highways
  // Each highway is one line now instead of a bunch of points. Clicking anywhere
  // on a line gets the weather there, or picks it as a route start/end
  fetch("/api/highways")
    .then((r) => r.json())
    .then((highways) => {
      highways.features.forEach((f) => {
        const name = f.properties.name;
        const line = L.geoJSON(f, {
          style: { color: highwayColors[name], weight: 5, opacity: 0.8 },
        }).addTo(map);
        line.bindTooltip(name, { sticky: true });
        // treat the line like a marker so the filters work the same way
        line.pointData = { point_type: "highway", name };
        allMarkers.push(line);
        line.on("click", (e) => {
          const p = {
            name,
            lat: +e.latlng.lat.toFixed(4),
            lng: +e.latlng.lng.toFixed(4),
          };
          isPlanning ? handleRouteSelection(p) : openWeatherForPoint(p);
        });
      });
      // Default to showing only highways not all the POI
      applyFilters();
    });

  // ---------------------------------------------------- fetch and plot all the Points of interest w/ icon png
  fetch("/api/pois")
    .then((r) => r.json())
    .then((pois) => {
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
      // GeoJSON coordinates are [lng, lat], so flip them for leaflet
      pois.features.forEach((f) => {
        const [lng, lat] = f.geometry.coordinates;
        const p = { ...f.properties, point_type: f.properties.type, lat, lng };
        const marker = L.marker([lat, lng], { icon: icons[p.type] }).addTo(map);
        marker.bindTooltip(p.name);
        marker.pointData = p;
        allMarkers.push(marker);
        marker.on("click", () =>
          isPlanning ? handleRouteSelection(p) : openWeatherForPoint(p),
        );
      });
      applyFilters();
    });

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
  // The server does the routing now: it snaps both points to the nearest highway
  // with PostGIS, runs Dijkstra in Go, and sends back the route line plus weather
  // checkpoints spaced every 15 miles
  function handleRouteSelection(clickedPoint) {
    if (!startPoint) {
      startPoint = clickedPoint;
      document.getElementById("status").innerHTML =
        `START SET: ${clickedPoint.name}. <span class="text-secondary">CLICK DESTINATION</span>`;
      return;
    }
    document.getElementById("status").textContent = "FINDING ROUTE...";
    fetch("/api/routes", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        from: { lat: startPoint.lat, lng: startPoint.lng },
        to: { lat: clickedPoint.lat, lng: clickedPoint.lng },
      }),
    })
      .then((r) => r.json())
      .then((route) => {
        if (route.error) {
          document.getElementById("status").textContent = route.error;
          startPoint = null;
          return;
        }
        isPlanning = false;
        routeInfo = route;
        // each checkpoint gets a name so the status bar says where you are
        activeRoute = route.checkpoints.map((c) => ({
          ...c,
          name: `Mile ${Math.round(c.mile)} · ${c.highway}`,
        }));
        startWalkthrough();
      });
  }

  // ---------------------------------------------------- Walk Through Functionality start
  function startWalkthrough() {
    // hides all other markers so route is distinct, but keep the points of interest along the way
    hideAllMarkers();
    const nearbyIds = new Set(routeInfo.nearby.map((p) => p.id));
    allMarkers
      .filter((m) => nearbyIds.has(m.pointData.id))
      .forEach((m) => m.addTo(map));

    // make the next, prev and auot play button visuable
    document.getElementById("route-nav").classList.remove("d-none");
    // disable the play button while weather info loads
    document.getElementById("autoplay-btn").disabled = true;
    // draw the route line, it comes back from the server as GeoJSON
    if (routeLine) map.removeLayer(routeLine);
    routeLine = L.geoJSON(routeInfo.line, {
      style: { color: "blue", weight: 6, opacity: 0.5 },
    }).addTo(map);
    map.fitBounds(routeLine.getBounds(), { padding: [40, 40] });

    // distance, highways, and the towns and passes you go through
    const along = routeInfo.nearby
      .filter((p) => p.type !== "sasquatch")
      .map((p) => p.name);
    document.getElementById("route-summary").textContent =
      `${Math.round(routeInfo.distance_miles)} miles · ${routeInfo.highways.join(" → ")}` +
      (along.length ? ` · Through ${along.join(", ")}` : "");
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
      `CHECKPOINT ${currentIndex + 1} OF ${activeRoute.length}`;
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
    routeInfo = null;
    document.getElementById("route-summary").textContent = "";
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
    // else remove spinner, and fetch the weather from /api/weather, parse response and call render Weather
    document.getElementById("weather-spinner").classList.remove("d-none");
    document.getElementById("weather-content").innerHTML = "";
    fetch(`/api/weather?lat=${point.lat}&lng=${point.lng}`)
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
        fetch(`/api/weather?lat=${point.lat}&lng=${point.lng}`)
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
