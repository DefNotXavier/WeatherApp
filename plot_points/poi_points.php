<?php
require '../db_conn.php';

header('Content-Type: application/json');
// all POI are ones that are not intermediate, junction or border
$result = $conn->query("SELECT * FROM points WHERE point_type NOT IN ('intermediate', 'junction', 'border') ORDER BY point_type");
$points = $result->fetch_all(MYSQLI_ASSOC);

echo json_encode($points);
?>