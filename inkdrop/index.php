<?php
session_start();

include "guard.php";

$baseRepoDir = __DIR__ . "/repos/";
$username = $_SESSION["name"];

$viewPublic = ($_GET["view"] ?? "") === "public";
$publicUser = $_GET["user"] ?? null;

$userRepoDir = "$baseRepoDir/$username/";
if (!is_dir($userRepoDir)) {
    mkdir($userRepoDir, 0775, true);
}

$createMessage = "";

if (
    !$viewPublic &&
    $_SERVER["REQUEST_METHOD"] === "POST" &&
    isset($_POST["reponame"])
) {
    $reponame = trim($_POST["reponame"]);
    if (!preg_match("/^[a-zA-Z0-9_-]{1,50}$/", $reponame)) {
        $createMessage =
            "<b style='color: red'>Repository name contains invalid characters or is too long.</b>";
    } else {
        $fullRepoPath = "$userRepoDir$reponame/";
        if (is_dir($fullRepoPath)) {
            $createMessage =
                "<b style='color: red'>Repository already exists.</b>";
        } else {
            mkdir($fullRepoPath, 0755, true);
            $createMessage =
                "<b style='color: #00FF00'>Repository created successfully.</b>";
        }
    }
}
?>
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
        <title>InkDrop</title>
    </head>
    <body>
        <main>
            <h1 class="intro">InkDrop Main Page, written by Quan Thai</h1>
            <br /><hr class="linebreaker"/><br /><br>
            <?php include "connect.php"; ?>
            <div class="main1">
                <?php if (!$viewPublic):?>
                <p style='font-size: 25pt'>Logged in as <b><?php echo htmlspecialchars($username); ?></b></p>
                <nav>
                    <a href="?view=public"><button class="select">Browse Repositories</button></a>
                </nav>
                <br /><hr class="linebreaker"/><br />
                <form method="POST" action="index.php">
                    <input name="reponame" type="text" class="details" placeholder="Enter new repository name..." required/><br /><br />
                    <button type="submit" class="redirect">Create Repo</button>
                </form>

                <?php $createMessage ? "<br><br>$createMessage" : ""?>
            </div>
            <div class="main2">
                <h2>Your Repositories:</h2>
                <ul>
                    <?php
                    $repos = array_filter(glob($userRepoDir . "*"), "is_dir");
                    if (empty($repos)) {
                        echo "<li>You do not have any repositories. Consider creating one to store files.</li>";
                    } else {
                        foreach ($repos as $repoPath) {
                            $repoName = basename($repoPath);
                            echo "<li><a href='repo.php?name=$repoName&user=$username'>$repoName</a></li>";
                        }
                    }
                    ?>
                </ul>
                <br /><br />
                <a href="logout.php"><button class="redirect">Logout</button></a>
            </div>
        </main>
    </body>
    <style>
    main {
        background-image: linear-gradient(
            to bottom,
            var(--primary),
            var(--secondary)
        );
        width: 100%;
        height: 100vh;
        position: relative;
        display: flex;
        flex-direction: column;
        justify-content: center;
        word-break: break-word;
    }

    * {
        scrollbar-width: none;
    }
    </style>
</html>
