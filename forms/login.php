<?php
session_start();
// Had to use __DIR__ to bypase all the relative paths and use the absolute path
require __DIR__ . '/../db_conn.php';

if ($_SERVER['REQUEST_METHOD'] === 'POST') {
    $username = trim($_POST['username']);
    $password = trim($_POST['password']);

    $stmt = $conn->prepare("SELECT id, username, password_hash FROM users WHERE username = ?");
    $stmt->bind_param("s", $username);
    $stmt->execute();
    $result = $stmt->get_result();
    $user = $result->fetch_assoc();
    $stmt->close();

    if ($user && password_verify($password, $user['password_hash'])) {
        $_SESSION['user_id'] = $user['id'];
        $_SESSION['username'] = $user['username'];
        header("Location: ../history_lookup.php");
        exit;
    } else {
        $error = "Invalid username or password.";
        header("Location: /WeatherApp/index.php?error=1");
        exit;
    }
}
?>

<div id="popupLogin">
  <?php if (!empty($error)): ?>
    <div class="alert alert-danger small"><?= htmlspecialchars($error) ?></div>
  <?php endif; ?>
  <form method="POST" action="forms/login.php">
    <h3 class="text-center mb-3">Admin Login</h3>
    <div class="mb-3">
      <label class="form-label">Username</label>
      <input type="text" name="username" class="form-control" placeholder="Admin" required maxlength="50">
    </div>
    <div class="mb-3">
      <label class="form-label">Password</label>
      <input type="password" name="password" class="form-control" required maxlength="50" placeholder="********">
    </div>
    <button type="submit" class="btn btn-dark w-100 mb-2">Sign In</button>
    <button id="closeLoginBtn" type="button" class="btn btn-dark w-100 mb-2">Close</button>
  </form>
</div>
  <!-- This will show the error in a div back in index.php, also boot strap has a automatic x to get ride of the alert -->
<?php if (!empty($_GET['error'])): ?>
  <div class="alert alert-danger small">Invalid username or password
     <button type="button" class="btn-close float-end" data-bs-dismiss="alert"></button>
  </div>
<?php endif; ?>