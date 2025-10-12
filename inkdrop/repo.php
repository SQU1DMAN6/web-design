<?php
include "guard.php";

$repo = $_GET["name"] ?? null;
$user = $_GET["user"] ?? ($_SESSION["name"] ?? null);

if (!$repo || !$user) {
    echo "Repository or user is not specified. Please proceed to <a href='index.php'>the main page</a>.";
    exit();
}

$repoPath = __DIR__ . "/repos/$user/$repo";

if (!is_dir($repoPath)) {
    echo "The repository is not found. Please proceed to <a href='index.php'>the main page</a>.";
    exit();
}

$isOwner = ($_SESSION["name"] ?? null) === $user;

// Handle file delection, but only available to the owner of the repository
if ($isOwner && isset($_GET["delete"])) {
    $fileToDelete = basename($_GET["delete"]);
    $filePath = "$repoPath$fileToDelete";

    if (is_file($filePath)) {
        unlink($filePath);
        header("Location: repo.php?name=" . urlencode($repo));
        exit();
    }
}

// Handle file upload, but only available to the owner of the repository

if (
    $isOwner &&
    $_SERVER["REQUEST_METHOD"] === "POST" &&
    isset($_FILES["upload"])
) {
    $file = $_FILES["upload"];
    $target = $repoPath . basename($file["name"]);

    if (move_uploaded_file($file["tmp_name"], $target)) {
        $message =
            "<b style='color: #0f0'>Uploaded" .
            htmlspecialchars($file["name"]) .
            ".</b>";
    } else {
        $message = "<b style='color: red'>Upload failed.</b>";
    }
}

// Handle file preview, which is available to all users
$previewContent = "";

if (isset($_GET["preview"])) {
    $previewFile = basename($_GET["preview"]);
    $previewPath = "$repoPath$previewFile";

    $finfo = finfo_open(FILEINFO_MIME_TYPE);
    $mime_type = finfo_file($finfo, $filePath);
    finfo_close($finfo);

    if (is_file($previewFile)) {
        $finfo = finfo_open(FILEINFO_MIME_TYPE); // Return MIME type
        $mime_type = finfo_file($finfo, $previewPath);
        finfo_close($finfo);

        if ($mime_type !== false) {
            $main_type = strtok($mime_type, "/"); // Get the part before the '/' in the MIME type

            switch ($main_type) {
                case "video":
                    $previewContent =
                        "<h3>Preview of " .
                        htmlspecialchars($previewFile) .
                        "</h3>" .
                        "<br><br>" .
                        "<video width='90%' controls><source src='$previewPath' type='$mime_type'>Your browser does not support HTML video.</video>";
                    break;
                case "audio":
                    $previewContent =
                        "<h3>Preview of " .
                        htmlspecialchars($previewFile) .
                        "</h3>" .
                        "<br><br>" .
                        "<audio controls><source src='$previewPath' type='$mime_type'>Your browser does not support the audio element.</audio>";
                    break;
                case "image":
                    $previewContent =
                        "<h3>Preview of" .
                        htmlspecialchars($previewFile) .
                        "</h3>" .
                        "<br><br>" .
                        "<img src='$previewPath'>";
                    break;
                case "text":
                    $content = file_get_contents($previewPath);
                    $previewContent =
                        "<h3>Preview of" .
                        htmlspecialchars($previewPath) .
                        "</h3>" .
                        "<br><br>" .
                        "<pre>" .
                        htmlspecialchars($content) .
                        "</pre>";
                default:
                    $allowedExts = [
                        "txt",
                        "md",
                        "json",
                        "conf",
                        "sh",
                        "php",
                        "js",
                        "css",
                        "html",
                        "py",
                        "cpp",
                        "go",
                        "cs",
                        "xml",
                    ]; // Previewable file extensions
                    $ext = pathinfo($previewFile, PATHINFO_EXTENSION);

                    if (in_array($ext, $allowedExts)) {
                        $content = file_get_contents($previewPath);
                        $previewContent =
                            "<h3>Preview of" .
                            htmlspecialchars($previewPath) .
                            "</h3>" .
                            "<br><br>" .
                            "<pre>" .
                            htmlspecialchars($content) .
                            "</pre>";
                    } else {
                        echo "File is not previewable.";
                    }
                    break;
            }
        }
    } else {
        echo "<p style='color: orange'>File is not previewable.</p>";
    }
}
?>
<!DOCTYPE html>
<html lang="en">
    <head>
        <meta charset="UTF-8" />
        <link rel="stylesheet" href="root.css?version=1.2" />
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
        <link
            href="https://fonts.googleapis.com/css2?family=Open+Sans:ital,wght@0,300..800;1,300..800&family=Source+Code+Pro:ital@0;1&display=swap"
            rel="stylesheet"
        />
        <title><?php htmlspecialchars($repo); ?> - InkDrop</title>
    </head>
    <body>
        <main>
            <h1 class="intro"><?php htmlspecialchars($repo); ?></h1>
        </main>
    </body>
    <style>
    * {
        scrollbar-width: none;
    }

    main {
        background-image: linear-gradient(
            to bottom,
            var(--primary),
            var(--secondary)
        );
        color: white;
        display: flex;
        flex-direction: column;
        overflow: auto;
        scrollbar-width: none;
        width: 100%;
        height: 100vh;
    }
    </style>
</html>
