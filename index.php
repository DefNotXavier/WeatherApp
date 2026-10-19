<?php
// Prevent URL manipulation by redirect to index.php
if (!empty($_SERVER['PATH_INFO'])) {
    header("Location: /WeatherApp/index.php");
    exit;
}
error_reporting(E_ALL);
ini_set('display_errors', 1);

// Start the session up here before any HTML gets sent, otherwise
// forms/login.php throws "headers already sent" in Docker
if (session_status() === PHP_SESSION_NONE) {
    session_start();
}

?>
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Montana Route Planner</title>
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/leaflet/1.9.4/leaflet.min.css" />
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/css/bootstrap.min.css" />
    <link rel="stylesheet" href="/WeatherApp/styles.css" />
</head>
<body>
<!-- Login and filter popups are hidden until button presses -->
 <?php include 'forms/login.php'; ?>
 <?php include 'forms/filter.php'; ?>
<div id="app">
    <nav class="navbar navbar-dark bg-dark px-3 flex-shrink-0">
        <span class="navbar-brand mb-0 h1">Montana Route Planner</span>
        <div class="d-none d-md-flex align-items-center gap-3 me-auto ms-4">
            <span class="badge" style="background:blue">I-90</span>
            <span class="badge" style="background:red">I-15</span>
            <span class="badge" style="background:green">I-94</span>
        </div>
        <div class="d-flex gap-2">
            <button id="plan-btn" class="btn btn-outline-light btn-sm">Plan a Route</button>
           <button id="openLoginBtn" class="btn btn-outline-light btn-sm">Admin Login</button>
           <button id="openFilterBtn" class="btn btn-outline-light btn-sm">Filter</button>
        </div>
    </nav>
    <!-- Leaflet map container -->
    <div id="map"></div>

    <!-- Slide-up bottom panel for weather and route info -->
    <div id="bottom-panel">
        <div id="pull-tab">
            <span id="arrow" class="me-2">▲</span> More Info
        </div>
        <div id="panel-content" class="d-none">
            <!-- Route nav btns are hidden until route is planned -->
            <div id="route-nav" class="d-none">
            <div class="d-flex align-items-center justify-content-between px-2">
                <button id="prev-btn" class="btn btn-dark" style="font-size: 2rem; line-height:1; padding: 0.2rem 1rem;">←</button>
                <div class="d-flex flex-column align-items-center gap-1">
                    <div id="route-index-text" class="fw-bold small">POINT 1 OF 0</div>
                    <button id="autoplay-btn" class="btn btn-outline-dark btn-sm" disabled>▶ Autoplay</button>
                </div>
                <button id="next-btn" class="btn btn-dark" style="font-size: 2rem; line-height:1; padding: 0.2rem 1rem;">→</button>
            </div>
        </div>
            <!-- Progress bar shown while prefetching weather for all route points -->
            <div id="prefetch-bar" class="d-none px-3 py-2 bg-light border-bottom">
                <div class="small text-muted mb-1 text-center">Loading weather data for route...</div>
                <div class="progress" style="height: 6px;">
                    <div id="prefetch-progress" class="progress-bar bg-secondary" style="width: 0%"></div>
                </div>
            </div>
            <!-- Status bar-->
            <div id="status-bar" class="d-flex justify-content-center align-items-center py-2">
                <span id="status" class="px-3 py-1 rounded-pill border text-muted small fw-semibold">Please Click a Point</span>
            </div>
            <!-- Circle spinner-->
            <div id="weather-spinner" class="text-center p-4 d-none">
                <div class="spinner-border spinner-border-sm text-secondary"></div>
            </div>
            <!-- Paresed Weather cards -->
            <div id="weather-content"></div>
        </div>
    </div>
</div>

<script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/js/bootstrap.bundle.min.js"></script>
<script src="https://cdnjs.cloudflare.com/ajax/libs/leaflet/1.9.4/leaflet.min.js"></script>
<!-- this is the main app logic is app.js -->
<script src="/WeatherApp/app.js"></script>
</body>
</html>
