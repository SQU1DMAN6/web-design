<?php
// Force HTTPS
if (empty($_SERVER["HTTPS"]) || $_SERVER["HTTPS"] === "off") {
    header(
        "Location: https://" . $_SERVER["HTTP_HOST"] . $_SERVER["REQUEST_URI"],
    );
    exit();
}

// Include DB and utilities at the top
include "connect.php"; // ensure this does NOT echo "Connected successfully"
include "util.php";

$store = false;
$name = "";
$email = "";
$password_cooked = "";
$message = "";

if ($_SERVER["REQUEST_METHOD"] === "POST") {
    $nam = trim($_POST["name"] ?? "");
    $name = str_replace(" ", "_", $nam);
    $email = strtolower(trim($_POST["email"] ?? ""));
    $password_raw = $_POST["password"] ?? "";

    if (!preg_match("/^[a-zA-Z0-9_-]{1,20}$/", $name)) {
        $message = "Name '$name' contains invalid characters or is too long.";
    } elseif (!isValidEmail($email)) {
        $message = "Your email address is not correct!";
    } else {
        $password_cooked = password_hash($password_raw, PASSWORD_DEFAULT);
        $store = true;

        // Check if user exists
        $checkQuery =
            "SELECT * FROM fileshare.users WHERE name = $1 OR email = $2";
        $checkResult = pg_query_params($db_handle, $checkQuery, [
            $name,
            $email,
        ]);

        if (pg_num_rows($checkResult) > 0) {
            $message = "Username or email already exists.";
            $store = false;
        } else {
            // Insert new user
            $insertQuery =
                "INSERT INTO fileshare.users (name, email, password) VALUES ($1, $2, $3)";
            $result = pg_query_params($db_handle, $insertQuery, [
                $name,
                $email,
                $password_cooked,
            ]);

            if ($result) {
                $store = true;
                $message = "New record created successfully!";
            } else {
                $message = "Error inserting into database.";
                $store = false;
            }
        }
    }
}
?>
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<link rel="stylesheet" href="root.css">
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Open+Sans:ital,wght@0,300..800;1,300..800&family=Source+Code+Pro:ital@0;1&display=swap" rel="stylesheet">
<title>InkDrop Register</title>
<style>
main {
    background-image: linear-gradient(to bottom, var(--primary) 30%, var(--secondary), var(--light-blue));
    color: white;
    height: 100vh;
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;
    text-align: center;
    margin: 0;
    padding: 0;
}
div.register-box, div.d2s {
    background-image: linear-gradient(to bottom, var(--primary), var(--light-secondary), var(--secondary));
    border-radius: 15px;
    box-shadow: 0 0 15px 5px var(--light-light-blue);
    display: flex;
    align-items: center;
    justify-content: center;
    flex-direction: column;
    text-align: center;
    padding: 20px;
}
div.register-box {
    border: 2px solid #fff;
}
div.d2s {
    border: 1px solid #fff;
}
.details {
    width: 200px;
    padding: 5px;
    margin: 5px 0;
}
.redirect {
    padding: 5px 15px;
    margin-top: 10px;
}
</style>
</head>
<body>
<main>
    <!-- Registration Form -->
    <div class="register-box" style="display: <?= $store ? "none" : "flex" ?>">
        <h1>Register for an InkDrop account</h1>
        <hr class="linebreaker"><br>
        <form method="POST">
            <input class="details" type="text" name="name" placeholder="NAME" required><br>
            <input class="details" type="email" name="email" placeholder="EMAIL" required><br>
            <input class="details" type="password" name="password" placeholder="PASSWORD" required><br>
            <button type="submit" class="redirect">Register</button>
        </form>
        <p>If you already have an account: <a href="login.php"><button class="redirect">Login</button></a></p>
        <?php if (!$store && $message): ?>
            <p style="color: orange;"><?= htmlspecialchars($message) ?></p>
        <?php endif; ?>
    </div>

    <!-- Success Message -->
    <div class="d2s" style="display: <?= $store ? "flex" : "none" ?>">
        <h1>Registration Successful!</h1>
        <p><?= htmlspecialchars($message) ?></p>
        <p>Hello, <?= htmlspecialchars($name) ?>!</p>
        <p>Your email: <?= htmlspecialchars($email) ?></p>
        <p>Your password is stored encrypted (hashed) in the database.</p>
        <p>Go to <a href="login.php"><button class="redirect">Login</button></a></p>
    </div>
</main>
</body>
</html>
