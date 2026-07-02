<?php
header('Content-Type: application/json');


// round the points
$lat = round((float)$_GET['lat'], 4);
$lng = round((float)$_GET['lng'], 4);

function nws_get($url) {
    $ch = curl_init($url);
    curl_setopt_array($ch, [
        CURLOPT_RETURNTRANSFER => true,
        CURLOPT_SSL_VERIFYPEER => false,  
        // Just to be clear, maybe its why im getting so many errors but i dont need a real email
        CURLOPT_HTTPHEADER => [
            'User-Agent: MontanaRoutePlanner/1.0 (soIactuallydontNeedaRealEmailHere@email.com)',
            'Accept: application/geo+json'
        ],
        CURLOPT_TIMEOUT => 10,
    ]);
    $body = curl_exec($ch);
    curl_close($ch);
    return $body ? json_decode($body, true) : null;
}
// get grid point
$meta = nws_get("https://api.weather.gov/points/{$lat},{$lng}");
if (!$meta || !isset($meta['properties']['forecast'])) {
    echo json_encode([
        'error' => 'Could not get NWS grid point',
        'nws_response' => $meta  
    ]);
    exit;
}

// get forecast
$forecast = nws_get($meta['properties']['forecast']);
if (!$forecast || !isset($forecast['properties']['periods'])) {
    echo json_encode(['error' => 'Could not get forecast']);
    exit;
}

// Return next 4 periods
echo json_encode(array_slice($forecast['properties']['periods'], 0, 4));
?>