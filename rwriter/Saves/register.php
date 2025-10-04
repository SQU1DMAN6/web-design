<?php
session_start();

if (isset($_SESSION["login"])) {
    header("Location: index.php");
    exit();
}

if (empty($_SERVER["HTTPS"]) || $_SERVER["HTTPS"] === "off") {
    header(
        "Location: https://" . $_SERVER["HTTP_HOST"] . $_SERVER["REQUEST_URI"],
    );
    exit();
}
?>

<!DOCTYPE html>
<html lang="en">

<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>RWRiter Register</title>
</head>

<body>
    <main>
        <div class="title"><h1>RWRiter Register</h1></div>
        <form action="register.php" method="POST" name="register">
            <label for="name">Full Name:</label>
            <input type="text" name="name" id="name" placeholder="Your Name" required><br><br>

            <label for="email">Email:</label>
            <input type="email" name="email" id="email" placeholder="Your Email" required><br><br>

            <label for="password">Password:</label>
            <input type="password" name="password" id="password" placeholder="Your Password" required><br><br>

            <button type="submit">Register</button>
        </form>
        <br><br>

        <p>If you already have an account, <a href="login.php"><button>Login</button></a></p><br><br>

        <?php
        echo "<br><br>";

        include "connect.php";
        include "util.php";

        $store = false;

        // Displaying form results once submitted
        if (isset($_POST["name"])) {
            $name = $_POST["name"];

            $email = strtolower(trim($_POST["email"]));
            if (!isValidEmail($email)) {
                echo "<span class='error'>Your email address is not correct!</span>";
                return;
            }

            $password_raw = $_POST["password"];
            $password_cooked = password_hash($password_raw, PASSWORD_DEFAULT);
            echo "<span class='success'>Hello, " .
                $name .
                "<br><br>Your email is: " .
                $email .
                "<br><br>Your password is encrypted:</span><br>" .
                $password_cooked .
                "<br><br>";
            $store = true;
        }

        // Validate information before we proceed. (for example use regex to validate email format)
        if ($store) {
            $checkQuery =
                "SELECT * FROM fileshare.users WHERE name = $1 OR email = $2";
            $checkResult = pg_query_params($db_handle, $checkQuery, [
                $name,
                $email,
            ]);

            if (pg_num_rows($checkResult) > 0) {
                echo "<span class='error'>Username or email already exists.</span><br>";
                $store = false;
            } else {
                $query =
                    "INSERT INTO fileshare.users (name, email, password) VALUES ($1, $2, $3)";
                $result = pg_query_params($db_handle, $query, [
                    $name,
                    $email,
                    $password_cooked,
                ]);

                if ($result) {
                    echo "<span class='success'>New record created!<br><br>Please proceed to <a href='login.php'><button>Login</button></a></span>";
                } else {
                    echo "<span class='error'>Error creating record.</span><br>";
                }
            }
            $store = false;
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
        overflow: auto;
    }

    main {
        padding: 20px;
        border-radius: 10px;
        background-color: #111;
        box-shadow: 0 0 10px 5px rgba(0, 255, 0, 0.2);
        text-align: center;
        width: 80%;
        max-width: 100%;
        border: 2px solid #00FF00;
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        overflow: none;
        flex-wrap: wrap;
    }

    .title {
        font-family: "Source Code Pro", monospace;
        width: 100%;
        color: #00FF00;
        background-color: #444;
    }

    h1 {
        font-size: 2em;
        margin-bottom: 20px;
        color: #00FF00;
    }

    label {
        color: #00FF00;
        font-size: 1.1em;
        margin-bottom: 5px;
        display: inline-block;
    }

    input {
        font-family: "Source Code Pro", monospace;
    }

    input[type="text"], input[type="email"], input[type="password"] {
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

    .success {
        color: #00FF00;
        background-color: black;
        padding: 5px;
        border-radius: 5px;
        font-weight: bold;
    }
</style>

</html>
