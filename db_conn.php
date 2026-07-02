
<?php
$host = 'localhost';
$db = 'montana_planner';
$user = 'montanauser';
$pass = 'MontanaPass$';

$conn = new mysqli($host, $user, $pass, $db);

if ($conn->connect_error) {
    die("Connection failed: " . $conn->connect_error);
}
?>