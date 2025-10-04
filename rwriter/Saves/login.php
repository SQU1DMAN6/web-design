<?php
session_start();
if (empty($_SERVER["HTTPS"]) || $_SERVER["HTTPS"] === "off") {
    header(
        "Location: https://" . $_SERVER["HTTP_HOST"] . $_SERVER["REQUEST_URI"],
    );
    exit();
}
if (isset($_SESSION["login"]) && isset($_SESSION["name"])) {
    header("Location: index.php");
    exit();
}
?>
<!DOCTYPE html>
<html lang="en">

<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>RWRiter Login</title>
</head>

<body>
    <main>
        <div class="title"><h1>RWRiter Login</h1></div>
        <form method="POST" name="login" action="login.php">
            <label for="email">Email:</label>
            <input type="email" name="email" id="email" required placeholder="Email" autocomplete="off"><br><br>
            <label for="password">Password:</label>
            <input type="password" name="password" id="password" required placeholder="Password" autocomplete="off"><br><br>
            <button type="submit">Login</button><br><br>
        </form>
        <br><br>
        <a href="register.php"><button>Register for RWRiter account</button></a>

        <?php
        include "connect.php";

        if (isset($_POST["email"]) && isset($_POST["password"])) {
            $email = trim($_POST["email"]);
            $password = $_POST["password"];

            $query = "SELECT * FROM fileshare.users WHERE email = $1";
            $result = pg_query_params($db_handle, $query, [$email]);

            if (!$result) {
                echo "Query failed: " . pg_last_error($db_handle);
                exit();
            }

            if (pg_num_rows($result) > 0) {
                $row = pg_fetch_assoc($result);
                if (password_verify($password, $row["password"])) {
                    $_SESSION["login"] = true;
                    $_SESSION["name"] = $row["name"];
                    $_SESSION["email"] = $row["email"];
                    session_regenerate_id(true);
                    echo "<script>window.location.href = 'index.php';</script>";
                    exit();
                } else {
                    echo "<br><br><b class='error'>Error logging in.</b><br><br>";
                }
            } else {
                echo "<br><br><b class='error'>Error logging in.</b><br><br>";
            }
        }
        ?>
    </main>
</body>

<style>
    * {
        margin: 0;
        padding: 0;
        box-sizing: border-box;
    }

    body {
        font-family: "Source Code Pro", monospace;
        background-color: #1a1a1a;
        color: #00FF00;
        display: flex;
        justify-content: center;
        align-items: center;
        height: 100vh;
        overflow: hidden;
    }

    main {
        padding: 20px;
        border-radius: 10px;
        background-color: #111;
        box-shadow: 0 0 10px 5px rgba(0, 255, 0, 0.2);
        text-align: center;
        width: 400px;
        max-width: 100%;
        border: 2px solid #00FF00;
    }

    h1 {
        font-size: 2em;
        margin-bottom: 20px;
        color: #00FF00;
    }

    input {
        font-family: "Source Code Pro", monospace;
    }

    input[type="email"], input[type="password"] {
        width: 100%;
        padding: 10px;
        margin: 10px 0;
        border: 2px solid #00FF00;
        border-radius: 5px;
        background-color: #222;
        color: #00FF00;
        font-size: 1em;
    }

    button {
        padding: 10px;
        width: 100%;
        background-color: #333;
        color: #00FF00;
        border: 2px solid #00FF00;
        border-radius: 5px;
        font-size: 1.1em;
        cursor: pointer;
        transition: background-color 0.3s ease;
        font-family: "Source Code Pro", monospace;
    }

    button:hover {
        background-color: #444;
    }

    a button {
        width: auto;
        display: inline-block;
        margin-top: 20px;
    }

    .error {
        color: #FF0000;
        background-color: black;
        padding: 5px;
        border-radius: 5px;
        font-weight: bold;
    }
</style>

</html>
