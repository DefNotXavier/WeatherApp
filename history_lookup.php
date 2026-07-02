<?php

session_start();
require 'db_conn.php';

// Have to be logged in to get to this page
if (!isset($_SESSION['user_id'])) {
    header("Location: index.php");
    exit;
}

$stmt = $conn->prepare("SELECT start_lat, start_lng, start_highway, end_lat, end_lng, end_highway, route_length, searched_at FROM search_log ORDER BY searched_at DESC");
$stmt->execute();
$logs = $stmt->get_result()->fetch_all(MYSQLI_ASSOC);
$stmt->close();
?>


<!DOCTYPE html>
<html>
<head>
  <title>Route History</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/css/bootstrap.min.css" />
  <link rel="stylesheet" href="/WeatherApp/styles.css" />
</head>
<body class="bg-light">
<div class="container py-4">


  <div class="card border-0 shadow-sm">
    <div class="card-header bg-dark text-white d-flex align-items-center">
    <span class="navbar-brand mb-0 h1 me-3">Montana Route Planner — <?= htmlspecialchars($_SESSION['username']) ?></span>
    <div class="ms-auto">
        <a href="logout.php" class="btn btn-outline-light btn-sm">Logout</a>
    </div>
</div>
    <div class="table-responsive">
      <table class="table table-striped table-hover mb-0">
        <thead class="table-light">
          <tr>
            <th>#</th>
            <th>Start (lat, lng)</th>
            <th>Start Highway</th>
            <th>End (lat, lng)</th>
            <th>End Highway</th>
            <th>Points</th>
            <th>Searched At</th>
          </tr>
        </thead>
        <body>
            <?php foreach ($logs as $i => $log): ?>
              <tr>
                <!-- Only real cool thing I did is color coordinate the Highway badges -->
                <td class="text-muted"><?= $i + 1 ?></td>
                <td>(<?= htmlspecialchars($log['start_lat']) ?>, <?= htmlspecialchars($log['start_lng']) ?>)</td>
                <td><span class="badge" style="background:<?= ['I-90'=>'blue','I-15'=>'red','I-94'=>'green'][$log['start_highway']] ?? 'gray' ?>"><?= htmlspecialchars($log['start_highway']) ?></span></td>
                <td>(<?= htmlspecialchars($log['end_lat']) ?>, <?= htmlspecialchars($log['end_lng']) ?>)</td>
                <td><span class="badge" style="background:<?= ['I-90'=>'blue','I-15'=>'red','I-94'=>'green'][$log['end_highway']] ?? 'gray' ?>"><?= htmlspecialchars($log['end_highway']) ?></span></td>
                <td><?= htmlspecialchars($log['route_length']) ?></td>
                <td class="text-muted small"><?= htmlspecialchars($log['searched_at']) ?></td>
              </tr>
            <?php endforeach; ?>
        </body>
      </table>
    </div>
  </div>

</div>
<script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/js/bootstrap.bundle.min.js"></script>
</body>
</html>