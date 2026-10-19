<?php
if (session_status() === PHP_SESSION_NONE) {
    session_start();
}

require 'db_conn.php';

$data = json_decode(file_get_contents("php://input"), true);
if (!$data) { http_response_code(400); exit; }

$stmt = $conn->prepare("
    INSERT INTO search_log (start_lat, start_lng, start_highway, end_lat, end_lng, end_highway, route_length)
    VALUES (?, ?, ?, ?, ?, ?, ?)
");
$stmt->bind_param("ddsddsi",
    $data['start_lat'],
    $data['start_lng'],
    $data['start_highway'],
    $data['end_lat'],
    $data['end_lng'],
    $data['end_highway'],
    $data['route_length']
);
$stmt->execute();
$stmt->close();
http_response_code(200);