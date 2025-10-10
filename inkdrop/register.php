<!DOCTYPE html>
<html lang="en">
    <head>
        <meta charset="utf-8">
        <link rel="stylesheet" href="root.css" />
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
        <link
            href="https://fonts.googleapis.com/css2?family=Open+Sans:ital,wght@0,300..800;1,300..800&family=Source+Code+Pro:ital@0;1&display=swap"
            rel="stylesheet"
        />
        <title>InkDrop Register</title>
    </head>
    <body>
        <main>
            <h1 class='intro'>Register for an InkDrop account</h1>
            <br><hr style="margin: 0 auto; width: 80%; line-height: 5px" /><br>
            <form action="register.php" method="POST" name="login">
                <input class="details" type="text" name="name" placeholder="NAME" required /><br />
                <input class="details" type="email" name="email" placeholder="EMAIL" required /><br />
                <input class="details" type="password" name="password" placeholder="PASSWORD" required /><br />
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
                    $name = $_POST["name"];
                    $name_final = str_replace(" ", "_", $name);

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
                    echo "Hello, " . $name_final . "<br><br>";
                    echo "Your email is: " . $email . "<br><br>";
                    echo "Your password is: " .
                        $password_raw .
                        "<br>Encrypted to: " .
                        $password_cooked .
                        "<br><br>";
                    $store = true;
                }

                if ($store) {
                    $checkQuery =
                        "SELECT * FROM fileshare.users WHERE name = $1 OR email = $2";
                    $checkResult = pg_query_params($db_handle, $checkQuery, [
                        $name_final,
                        $email,
                    ]);

                    if (pg_num_rows($checkResult) > 0) {
                        echo "<br><b style='color: orange;'>Username or email already exists.</b><br>";
                        $store = false;
                    } else {
                        $query =
                            "INSERT INTO fileshare.users (name, email, password) VALUES ($1, $2, $3)";
                        $result = pg_query_params($db_handle, $query, [
                            $name_final,
                            $email,
                            $password_cooked,
                        ]);

                        if ($result) {
                            echo "<br><br><b style='color: green;'>New record created.</b><br><br>";
                            echo "Please redirect to the login page: <a href='login.php'><button class='redirect'>Login</button></a>";
                        }
                    }
                }
                ?>
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
    </style>
</html>
