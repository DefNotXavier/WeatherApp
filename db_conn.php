<?php
// Uses the env vars from docker compose if they're set,
// otherwise falls back to the local AMPPS settings
$host = getenv('DB_HOST') ?: 'localhost';
$db   = getenv('DB_NAME') ?: 'montana_planner';
$user = getenv('DB_USER') ?: 'montanauser';
$pass = getenv('DB_PASS') !== false ? getenv('DB_PASS') : 'MontanaPass$';
$port = (int) (getenv('DB_PORT') ?: 3306);

$conn = new mysqli($host, $user, $pass, $db, $port);

if ($conn->connect_error) {
    die("Connection failed: " . $conn->connect_error);
}
?>
