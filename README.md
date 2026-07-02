# Montana Route Planner

A LAMP-stack web application that displays live weather forecasts along Montana's interstate highways (I-90, I-15, I-94), allowing users to plan routes and view real-time National Weather Service data along their path.

---

## Project File Structure

```
WeatherApp/
├── index.php               # Main application entry point
├── db_conn.php             # Database connection
├── get_weather.php         # NWS weather API integration
├── log_route.php           # Logs route searches to the database
├── history_lookup.php      # Authenticated route history page
├── logout.php              # Session logout
├── styles.css              # Custom styles built on top of Bootstrap
├── App.js                  # Main frontend logic
├── forms/
│   ├── login.php           # Admin login form
│   └── filter.php          # Map filter popup form
├── plot_points/
│   ├── highway_points.php  # Returns grouped highway points as JSON
│   ├── border_points.php   # Returns Montana border points as JSON
│   └── poi_points.php      # Returns points of interest as JSON
├── images/
│   ├── car.png             # Car icon for route planning
│   ├── city.png            # City map icon
│   ├── pass.png            # Mountain pass map icon
│   └── sasquatch.png       # Sasquatch sighting map icon
└── sql_setup/
    ├── db_setup.sql         # Creates the database, tables, and accounts
    └── insert_points.sql    # Inserts highway, border, and POI data points
```

---

## Features

**Public**

- Interactive Leaflet map of Montana's three interstate highways (I-90, I-94, I-15)
- Click any point to view its NWS weather forecast
- Filter map points by type (highways, cities, mountain passes, sasquatch sightings)
- Plan a route by selecting a start and end point on the map (requires an active filter)
- Step through the route's weather forecasts point by point, automatically or manually

**Authenticated**

- View the full history of logged route searches
- See start/end coordinates, highways used, route length, and timestamp for each search
- Data is pulled live from the `search_log` table in the `montana_planner` database

---

## Requirements

- [AMPPS](https://www.ampps.com/) (Apache, MySQL, PHP)
- An internet connection (required for live map tiles and weather data)

---

## Installation

### 1. Copy Project Files

Copy the `WeatherApp/` folder into your AMPPS web root, e.g.:

```
C:\Program Files\Ampps\www\WeatherApp\
```

### 2. Set Up the Database

Open phpMyAdmin:

```
http://localhost/phpmyadmin
```

- Go to the **SQL** tab, paste the full contents of `sql_setup/db_setup.sql`, and click **Go**
- Repeat with `sql_setup/insert_points.sql`
- Verify that the `montana_planner` database was created with three tables: `points`, `search_log`, and `users`
- Confirm the `points` table contains data and the `users` table has an `admin` account
- Under **User Accounts**, verify that a user named `montanauser` exists

### 3. Run the Application

```
http://localhost/WeatherApp/index.php
```

---

## Admin Login

An admin account is created automatically by `db_setup.sql`:

| Field    | Value      |
| -------- | ---------- |
| Username | `admin`    |
| Password | `admin123` |

Click **Admin Login** in the navbar to sign in. After logging in, you'll be redirected to the route history page showing all logged searches.

> Only a single admin account exists; there is no self-service account creation.

---

## Notes

- Weather data is fetched live from the National Weather Service API and may occasionally fail to return a result
- Bootstrap and Leaflet are loaded via CDN
- Route searches are logged to the database regardless of login status
- The history page requires authentication to view
- Individual route points are not logged — only completed route plans are

---

## Data Sources

- Highway point data derived from [OpenStreetMap](https://www.openstreetmap.org/) via [Geofabrik](https://www.geofabrik.de/)
- Montana border outline points from the PublicaMundi Mapping API
- Weather data from the [National Weather Service NDFD API](https://api.weather.gov/)

---

## Tech Stack

- **Backend:** PHP, MySQL
- **Frontend:** JavaScript, Leaflet.js, Bootstrap
- **APIs:** National Weather Service (NDFD), OpenStreetMap
