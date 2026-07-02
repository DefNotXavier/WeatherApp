---
marp: true
theme: default
paginate: true
footer: "CS 470 — Montana Route Planner | Xavier G. Stout"
style: |
  @import url("https://fonts.googleapis.com/css2?family=Inter:wght@400;700&display=swap");

  :root {
    font-family: "Work Sans Regular", Arial;
    --main-color: #2c3e50;
    --text-color: #2c3e50;
    --bg-color-alt: #ffffff;
    --mark-background: #aed6f1;
  }

  section {
    background-color: #ffffff;
    background-size: 20px 20px;
    background-image:
      linear-gradient(#2c3e5012 1px, transparent 1px),
      linear-gradient(to right, #2c3e5012 1px, #2c3e500a 1px);
  }

  h1, h2, h3, h4, h5, h6 {
    color: var(--text-color);
  }

  header {
    font-size: 0.7em;
    color: var(--text-color);
    border-bottom: 1px solid #2c3e50;
  }

  footer {
    font-size: 0.7em;
    color: var(--text-color);
    border-top: 1px solid #2c3e50;
  }

  code {
    background-color: #d5d8dc;
    font-size: 0.9em;
  }

  pre {
    background-color: #d5d8dc;
  }

  blockquote {
    background: #d5d8dc;
    border-left: 10px solid var(--main-color);
    margin: 0.5em;
    padding: 0.5em;
  }

  mark {
    background-color: #5dade2;
    padding: 0 2px 2px;
  }

  section::after {
    font-size: 0.75em;
    content: attr(data-marpit-pagination) " / " attr(data-marpit-pagination-total);
    color: var(--text-color);
  }

  table {
    display: block;
    margin: 0 auto;
  }

  th {
    background-color: #34495e;
    color: white;
  }

  section.tinytext > p,
  section.tinytext > ul,
  section.tinytext > blockquote {
    font-size: 0.65em;
  }

  img[alt~="center"] {
    display: block;
    margin: 0 auto;
  }

  section.title {
    background-color: #0d2137;
    background-image: none;
    color: white;
  }

  section.title h1, section.title h2, section.title h3, section.title p {
    color: white;
  }

  section.title footer {
    color: #a8c8e0;
    border-top: 1px solid #1a6b8a;
  }

  section.title section::after {
    color: #a8c8e0;
  }
---

<!-- _class: title -->

# Montana Route Planner

## CS 470 — Final Project Presentation

**Xavier G. Stout | April 2026**

`I-90` &nbsp; `I-15` &nbsp; `I-94` &nbsp; — &nbsp; Full LAMP-Stack Web Application

---

## Project Statement

> Help drivers make informed decisions before driving by showing **live NWS weather forecasts** along Montana's three interstate highways.

**The problem:** Montana roads span hundreds of miles with rapidly changing conditions. No single tool existed to view forecast data along specific interstate corridors before a drive.

**The solution:** A from-scratch LAMP-stack web application that lets any user plan a route and step through weather forecasts — point by point — from start to finish.

---

## Full LAMP-Stack Overview

| Layer | Technology       | Role                              |
| ----- | ---------------- | --------------------------------- |
| **L** | Linux / AMPPS    | Server environment                |
| **A** | Apache           | Serves PHP & static assets        |
| **M** | MariaDB / MySQL  | `montana_planner` database        |
| **P** | PHP 8            | Middleware, API, auth, JSON feeds |
| **+** | JavaScript ES6   | All frontend app logic (`app.js`) |
| **+** | Leaflet.js 1.9.4 | Interactive map rendering         |
| **+** | Bootstrap 5.3    | Responsive UI framework           |

> Developed exclusively with tools covered in our text — no Google Maps, no external routing APIs.

---

## Data & Database

Three tables in `montana_planner`:

- **`points`** — 200+ geo-points sourced from OpenStreetMap/Geofabrik
  - `highway`, `name`, `lat`, `lng`, `sequence_order`, `point_type`, `nearest_highway_pt`
  - Types: `intermediate`, `junction`, `border`, `city`, `pass`, `sasquatch`
- **`users`** — Admin credentials with bcrypt password hashes
- **`search_log`** — Every route search auto-logged: start/end coords, highways, route length, timestamp

**Dynamic JSON feeds** served via PHP endpoints:

- `highway_points.php` → grouped by highway
- `poi_points.php` → cities, passes, sasquatch sightings
- `border_points.php` → Montana outline polygon

---

## PHP Middleware

<!-- _class: tinytext -->

- **`get_weather.php`** — Two-step NWS API chain: `/points/{lat},{lng}` → `/forecast`. Returns 4 forecast periods as JSON. Coordinates rounded to 4dp to reduce API rejection rate.

- **`log_route.php`** — Accepts POST JSON, inserts to `search_log` using prepared statements on every route plan regardless of login status.

- **`login.php`** — Verifies bcrypt hash via `password_verify()`, sets `$_SESSION` on success, redirects to history page.

- **`history_lookup.php`** — Session-protected. Queries `search_log DESC`, renders Bootstrap table with color-coded highway badges.

- **`highway_points.php` / `poi_points.php` / `border_points.php`** — Serve DB data as JSON to the frontend fetch calls.

---

## Web Interface — Public Access

Any visitor (no login) can:

- View all **3 Montana interstates** color-coded on a Leaflet map (I-90 blue, I-15 red, I-94 green) with Montana border outline
- **Click any point** to fetch and display live NWS weather forecast cards
- **Filter** map markers by type: highway points, cities, mountain passes, sasquatch sightings
- **Plan a route** — select start & end points anywhere on the map
- **Step through** weather forecasts point by point with ← → buttons
- **Autoplay** — walks the route automatically every 2 seconds, moving a car marker on the map

---

## Web Interface — Authenticated Access

Admin login via navbar popup:

- Credentials verified against **bcrypt hash** stored in `users` table
- PHP `$_SESSION` manages auth state
- On success → redirected to **Route History** page
- Full log of all route searches with: start/end coordinates, highway badges, route length, timestamp
- **Logout** cleanly destroys session

> All route searches are logged to the database **regardless** of whether the user is logged in — authenticated users just get to _view_ the logs.

---

## Technical Highlights

**Multi-highway routing algorithm (`findRoute()`)**
Handles 0, 1, or 2 junction hops between interstates using ordered point arrays and spread/slice — no external routing library.

**Weather prefetch + progress bar**
`Promise.all()` fires parallel NWS requests for every route point. A live progress bar shows % complete before the walkthrough enables — real UX improvement over sequential fetching.

**Snap-to-highway for POI**
Cities, passes, and sasquatch sightings store a `nearest_highway_pt` FK. Clicking a POI for routing snaps it to the correct highway sequence point for accurate routing.

**NWS API error handling**
Occasional NWS failures are caught and rendered as styled error cards — the app never crashes silently.

---

## Peer Evaluation Results

| Category             | Max     | Jason D. | Rei K. | Nick S. |
| -------------------- | ------- | -------- | ------ | ------- |
| Deployment Quality   | 25      | 25       | 23     | 24      |
| Full LAMP Stack      | 35      | 32       | 33     | 34      |
| Meets Requirements   | 30      | 29       | 29     | 30      |
| Ease of Modification | 10      | 10       | 5      | 9       |
| **TOTAL**            | **100** | **96**   | **90** | **97**  |

**Average: 94.3 / 100**

> "You created a great application with some interesting methods." — Jason Decker

> "The autoplay feature works amazingly." — Rei Kepa

> "The weather prefetch progress bar is a good UX detail." — Nick Steilman

---

## Using Feedback

| Feedback                                   | From             | Action Taken                                                                                      |
| ------------------------------------------ | ---------------- | ------------------------------------------------------------------------------------------------- |
| Map points hard to select                  | Jason D.         | Increased `circleMarker` radius; POI icons enlarged to 28px                                       |
| Mobile view hard to read                   | Jason D.         | Expanded Bootstrap responsive utilities; resized bottom panel for narrow viewports                |
| Project nested in folders — path issue     | Rei K. & Nick S. | README updated to explicitly require placement directly in `www/` root                            |
| Logs require manual DB entry               | Rei K.           | Clarified: `log_route.php` auto-inserts on every route plan — no manual entry needed              |
| Clicking a point should auto-start routing | Nick S.          | Kept intentional — explicit "Plan a Route" button prevents accidental route starts while browsing |

---

## Originality & Learning

**Original features:**

- Sasquatch sightings layer with custom icon — fully toggleable
- Multi-highway junction routing (0/1/2-hop graph search)
- Parallel weather prefetch with live progress bar
- Snap-to-highway FK system for POI routing accuracy
- Color-coded highway badges in the admin route history table

**What I learned:**

- Consuming chained REST APIs (NWS two-step flow)
- Graph-style routing across ordered geographic datasets
- `Promise.all()` for parallel async fetching and UX performance
- PHP session security and bcrypt auth from scratch
- Managing complex application state in vanilla JS without a framework

---

<!-- _class: title -->

# Thank You

## Montana Route Planner — CS 470

**Full LAMP Stack · Live NWS Data · Multi-Highway Routing · Auth & Logging**

Average peer score: **94.3 / 100**

Questions?
