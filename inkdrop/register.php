<?php
if (empty($_SERVER["HTTPS"]) || $_SERVER["HTTPS"] === "off") {
    header(
        "Location: https://" . $_SERVER["HTTP_HOST"] . $_SERVER["REQUEST_URI"],
    );
    exit();
} ?>
<!DOCTYPE html>
<html lang="en">

<head>
    <meta charset="utf-8">
    <link rel="stylesheet" href="root.css" />
    <link rel="preconnect" href="https://fonts.googleapis.com" />
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
    <link
        href="https://fonts.googleapis.com/css2?family=Open+Sans:ital,wght@0,300..800;1,300..800&family=Source+Code+Pro:ital@0;1&display=swap"
        rel="stylesheet" />
    <title>InkDrop Register</title>
</head>

<body>
    <main>
        <div class="register-box">
            <h1 class='intro'>Register for an InkDrop account</h1>
            <br>
            <hr class="linebreaker" /><br>
            <form action="register.php" method="POST" name="login">
                <input class="details" type="text" name="name" placeholder="NAME" required /><br /><br />
                <input class="details" type="email" name="email" placeholder="EMAIL" required /><br /><br />
                <input class="details" type="password" name="password" placeholder="PASSWORD" required /><br /><br />
                <button type="submit" class="redirect">Register</button>
                <br /><br />
            </form>
            <p>If you already have an account, then <a href="login.php"><button class="redirect">Login</button></a></p>
            <?php
            echo "<br><br>";
            include "connect.php";
            include "util.php";

            $store = false;

            // Displaying form results once submitted
            if (isset($_POST["name"])) {
                $nam = trim($_POST["name"]);
                $name = str_replace(" ", "_", $nam);
                if (!preg_match("/^[a-zA-Z0-9_-]{1,20}$/", $name)) {
                    echo "Name '$name' contains invalid characters or is too long.";
                    return;
                }

                $email = strtolower(string: trim(string: $_POST["email"]));
                if (!isValidEmail(email: $email)) {
                    echo "Your email address is not correct!";
                    return;
                }

                $password_raw = $_POST["password"];
                $password_cooked = password_hash(
                    password: $password_raw,
                    algo: PASSWORD_DEFAULT,
                );
                $store = true;
            }
            ?>
        </div>
        <div class="d2s">
            <?php if ($store) {
                $checkQuery =
                    "SELECT * FROM fileshare.users WHERE name = \$1 OR email = \$2";
                $checkResult = pg_query_params($db_handle, $checkQuery, [
                    $name_final,
                    $email,
                ]);

                if (pg_num_rows($checkResult) > 0) {
                    echo "<br><b style='color: orange;'>Username or email already exists.</b><br>";
                    $store = false;
                } else {
                    $query =
                        "INSERT INTO fileshare.users (name, email, password) VALUES (\$1, \$2, \$3)";
                    $result = pg_query_params($db_handle, $query, [
                        $name_final,
                        $email,
                        $password_cooked,
                    ]);

                    if ($result) {
                        echo `
        <script>
        const divToHide = document.querySelector('.register-box')
        divToHide.style.display = 'hidden'
        const divToShow = document.querySelector('.d2s')
        divToShow.style.display = 'flex'
        divToShow.style.border = '1px solid #fff'
        </script>
        `;
                        echo "<p>";
                        echo "<br><br><b style='color: green;'>New record created.</b><br><br>";
                        echo "Hello, $name_final<br><br>";
                        echo "Your email is: $email<br><br>";
                        echo "Your password is: $password_raw<br>Encrypted to: $password_cooked<br><br>";
                        echo "Please redirect to the login page: <a href='login.php'><button class='redirect'>Login</button></a>";
                        echo "</p>";
                    }
                }
            } ?>
            <div>
    </main>
</body>
<style>
    main {
        background-image: linear-gradient(
            to bottom,
            var(--primary) 30%,
            var(--secondary),
            var(--light-blue)
        );
        color: white;
        height: 100vh;
        display: flex;
        flex-direction: column;
        justify-content: center;
        align-items: center;
        text-align: center;
        margin: 0px;
        padding: 0px;
    }

    div.register-box {
        background-image: linear-gradient(
            to bottom,
            var(--primary),
            var(--light-secondary),
            var(--secondary)
        );
        border: 2px solid #fff;
        border-radius: 15px;
        box-shadow: 0px 0px 15px 5px var(--light-light-blue);
        display: flex;
        align-items: center;
        justify-content: center;
        flex-direction: column;
    }

    div.d2s {
        background-image: linear-gradient(
            to bottom,
            var(--primary),
            var(--light-secondary),
            var(--secondary)
        );
        border-radius: 15px;
        box-shadow: 0px 0px 15px 5px var(--light-light-blue);
        display: hidden;
        align-items: center;
        justify-content: center;
        flex-direction: column;
        text-align: center;
    }
</style>

</html>
