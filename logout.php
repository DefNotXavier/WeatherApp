<?php
session_start();
session_unset();
session_destroy();
?>

<!DOCTYPE html>
<html>
<head>
  <title>Logged Out</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/css/bootstrap.min.css" />
</head>
<body class="bg-light">
<div class="container py-5 text-center">
  <div class="card border-0 shadow-sm mx-auto" style="max-width: 400px;">
    <div class="card-body py-5">
      <h4 class="mb-3">You have been logged out</h4>
      <a href="index.php" class="btn btn-dark">← Back to Map</a>
    </div>
  </div>
</div>
</body>
</html>