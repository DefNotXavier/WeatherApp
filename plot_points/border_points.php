<?php
require '../db_conn.php';

header('Content-Type: application/json');

$result = $conn->query("SELECT * FROM points WHERE point_type = 'border' ORDER BY sequence_order");
$points = $result->fetch_all(MYSQLI_ASSOC);


echo json_encode($points);
?>