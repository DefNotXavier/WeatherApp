<?php
require '../db_conn.php';

header('Content-Type: application/json');

$result = $conn->query("SELECT * FROM points WHERE highway != 'N/A' AND point_type IN ('intermediate', 'junction') ORDER BY highway, sequence_order");
$points = $result->fetch_all(MYSQLI_ASSOC);

$grouped = [];
foreach ($points as $p) {
    $grouped[$p['highway']][] = $p;
}

echo json_encode($grouped);
?>