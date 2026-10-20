# Montana Route Planner

![CI](https://github.com/DefNotXavier/WeatherApp/actions/workflows/ci.yml/badge.svg)

A web app for planning drives along Montana's interstates (I-90, I-15, I-94) and checking the National Weather Service forecast along the way. Pick a start and an end on the map, and it finds the shortest route, drops a weather checkpoint every 15 miles, and steps you through the forecast at each one.

The backend is a Go API on PostgreSQL + PostGIS with a Redis cache, and the whole thing runs with one `docker compose up`.

---

## Features

**Public**

- Interactive Leaflet map of Montana's three interstates, drawn from PostGIS line geometries
- Click anywhere on a highway, or on a city, pass, or sasquatch sighting, to see its forecast
- Filter the map by type (highways, cities, mountain passes, sasquatch sightings)
- Plan a route between any two spots. Clicks are snapped to the nearest highway, and the route can cross as many interstates as it needs to
- See the route's distance, the highways it uses, and the towns and passes you'll go through
- Step through the weather at each checkpoint, manually or on autoplay

**Admin**

- Log in to see every route that's been planned: start and end, highways, miles, number of checkpoints, and when it was searched

---

## How it's built

```mermaid
flowchart LR
    browser["Browser<br/>Leaflet + Bootstrap"]
    nws["National Weather<br/>Service API"]

    subgraph docker["Docker Compose"]
        app["<b>app</b><br/>Go API + static files<br/>:8080"]
        db[("<b>db</b><br/>PostgreSQL 16 + PostGIS<br/>:5433")]
        redis[("<b>redis</b><br/>Redis 7<br/>:6380")]
    end

    browser -- "GeoJSON, routes, weather" --> app
    app -- "geometry + spatial queries" --> db
    app -- "forecast cache, sessions" --> redis
    app -- "cache misses only" --> nws
```

| Container | What it does |
| --------- | ------------ |
| **app**   | Go server. Serves the frontend in `web/` and the JSON API, does the routing, and proxies NWS forecasts |
| **db**    | PostGIS database with the highway lines, state border, points of interest, users, and the route search log. Loads `db/` on first start |
| **redis** | Caches NWS responses and stores admin login sessions |

### Storing the map as geometry

The map data lives in PostGIS as real geometries (SRID 4326), not as rows of latitude and longitude:

| Table           | Geometry     | What's in it |
| --------------- | ------------ | ------------ |
| `highways`      | `LineString` | One line per interstate |
| `border`        | `Polygon`    | Montana's outline |
| `pois`          | `Point`      | Cities, mountain passes, sasquatch sightings |
| `search_log`    | `Point`      | Start and end of every planned route |
| `highway_nodes` | `Point`      | Materialized view of every highway vertex (`ST_DumpPoints`), used for routing |

That lets the database do the spatial work:

- **GeoJSON output.** `ST_AsGeoJSON` and `json_agg` build the FeatureCollections, and the Go server passes them straight to Leaflet.
- **Snapping.** A clicked point gets snapped to the nearest highway node with a KNN query (`ORDER BY geom <-> point`) that uses the GIST index. The first version needed a hand-entered `nearest_highway_pt` column on every point of interest, and that column is gone now.
- **What's along the route.** `ST_DWithin` on `geography` finds points of interest within 5 real miles of the route line, and `ST_LineLocatePoint` sorts them in the order you drive past them.

### Routing

On startup the server loads `highway_nodes` and builds a weighted graph in Go (`server/geo`):

- Every vertex is a node. Junctions like Butte (I-90/I-15) and Billings (I-90/I-94) are a vertex on both lines with the same coordinates, so they become one node that connects the highways.
- Vertices next to each other on a highway get an edge weighted by the distance between them in miles (haversine).

A route request snaps both ends, runs Dijkstra's algorithm with a binary heap, and then spaces weather checkpoints every 15 miles along the path. Before, every highway vertex was a checkpoint, and those are unevenly spaced: some a mile apart, some twenty.

Routing went through a few versions. The original JavaScript had hardcoded cases for zero, one, or two junctions and took the first match it found. I replaced that with Dijkstra in the browser, then moved it to Go once the server was there.

### Weather caching

Getting a forecast from NWS takes two requests: `/points/{lat},{lng}` returns the forecast grid cell, then the grid's forecast URL returns the forecast. A long route used to mean dozens of these on every search. Now they're cached in Redis:

| Cache key                | TTL        | Why |
| ------------------------ | ---------- | --- |
| `nws:point:{lat},{lng}`  | 7 days     | Grid cells don't move. Coordinates are rounded to ~1 km so nearby clicks share an entry |
| `nws:forecast:{url}`     | 30 minutes | NWS updates forecasts about hourly |

Concurrent requests for the same key are collapsed with `singleflight`, so a route that loads 34 checkpoints at once doesn't send duplicate calls to NWS. Server errors get one retry, and if Redis is down the app just goes straight to NWS.

Measured on a Helena → Glendive route (491 miles, 34 checkpoints): a cold checkpoint takes about **600 ms** because of the two NWS calls, and a cached one takes **under 1 ms**. Running the route a second time makes **0** NWS calls instead of 68. The `X-Cache: HIT/MISS` header on `/api/weather` shows which one you got.

---

## Running it

You need [Docker Desktop](https://www.docker.com/products/docker-desktop/) and an internet connection for the map tiles and weather.

```sh
git clone https://github.com/DefNotXavier/WeatherApp.git
cd WeatherApp
cp .env.example .env
docker compose up --build
```

The first start takes a minute while PostGIS loads `db/`. Then open **http://localhost:8080**.

Helper scripts, for Git Bash or a Mac/Linux terminal:

```sh
./bin/up            # build and start in the background, waits until healthy
./bin/down          # stop (database is kept)
./bin/down --wipe   # stop and delete the database
./bin/reset-db      # reload the database from db/ and restart the app
./bin/psql          # open a psql shell in the database
```

`web/` is mounted into the container, so HTML/JS/CSS edits show up on refresh. Go changes need `docker compose up --build`. The SQL in `db/` only runs the first time the database starts, so use `./bin/reset-db` after changing it.

### Admin login

| Username | Password   |
| -------- | ---------- |
| `admin`  | `admin123` |

Click **Admin Login** in the navbar. Sessions are random tokens stored in Redis with an 8 hour expiry, in an `HttpOnly` cookie.

---

## Tests

```sh
cd server
go test ./...                       # unit tests, no database needed

# with the stack running, this also runs the PostGIS integration tests
TEST_DATABASE_URL=postgres://montana:dev_db_pw@localhost:5433/montana_planner go test ./...
```

- `geo` covers the graph, Dijkstra (shortest path, multiple junctions, unreachable points), and checkpoint spacing
- `weather` runs against a fake NWS server: cache hits, sharing in-flight requests, retries, and Redis being down
- `api` covers the handlers with fake stores: route planning, input validation, login, and the history page
- `store` runs against the real database: GeoJSON output, snapping, and a full Helena → Glendive route across all three interstates

GitHub Actions runs gofmt, `go vet`, the whole test suite with the race detector against the real compose stack, and a smoke test of the running app on every push and pull request.

---

## API

| Method | Path            | Returns |
| ------ | --------------- | ------- |
| GET    | `/api/highways` | GeoJSON FeatureCollection of highway LineStrings |
| GET    | `/api/border`   | GeoJSON Polygon of Montana |
| GET    | `/api/pois`     | GeoJSON Points: cities, passes, sightings |
| POST   | `/api/routes`   | Plans and logs a route. Body: `{"from":{"lat":..,"lng":..},"to":{...}}` |
| GET    | `/api/weather?lat=..&lng=..` | Next 4 NWS forecast periods |
| GET    | `/healthz`      | Checks Postgres and Redis (used by the Docker healthcheck) |

A route response includes the snapped endpoints, `distance_miles`, the `highways` in driving order, the route as a GeoJSON `line`, the `checkpoints`, and the points of interest `nearby`. Both endpoints only accept coordinates in or near Montana, so `/api/weather` can't be used as an open NWS proxy.

---

## Project structure

```
WeatherApp/
├── server/                 Go API
│   ├── main.go             config and startup
│   ├── api/                HTTP handlers, login, history page template
│   ├── geo/                graph, Dijkstra, checkpoints (no dependencies)
│   ├── store/              PostGIS queries
│   ├── weather/            NWS client and Redis cache
│   ├── auth/               Redis sessions
│   └── Dockerfile          multi-stage build into a distroless image
├── web/                    frontend: index.html, app.js, styles.css, images
├── db/
│   ├── 01_schema.sql       tables, GIST indexes, admin user
│   ├── 02_data.sql         highways, border, and points of interest as geometries
│   └── 03_routing_nodes.sql
├── bin/                    helper scripts
├── compose.yaml
└── .github/workflows/ci.yml
```

---

## History

This started as my CSCI 470 project, a PHP/MySQL (LAMP) app that ran on AMPPS. Since then I've:

1. Containerized it with Docker Compose
2. Replaced the hardcoded junction routing with Dijkstra's algorithm
3. Moved the data into PostGIS geometries and rewrote the backend in Go with Redis caching, tests, and CI

The original PHP version is tagged [`v1-lamp`](https://github.com/DefNotXavier/WeatherApp/tree/v1-lamp).

---

## Data sources

- Highway geometry from [OpenStreetMap](https://www.openstreetmap.org/) via [Geofabrik](https://www.geofabrik.de/)
- Montana border from the PublicaMundi Mapping API
- Weather from the [National Weather Service API](https://www.weather.gov/documentation/services-web-api)

## Tech stack

- **Backend:** Go (net/http, pgx, go-redis)
- **Database:** PostgreSQL 16 + PostGIS 3.4
- **Cache and sessions:** Redis 7
- **Frontend:** JavaScript, Leaflet.js, Bootstrap
- **Infrastructure:** Docker, Docker Compose, GitHub Actions
