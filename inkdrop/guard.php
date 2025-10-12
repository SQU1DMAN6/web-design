<?php
session_start();

// Change session longevity to prevent users from being logged out constantly. Currently, the time has been set to 1 week, which is equivalent to 604800 seconds.
ini_set("session.gc_maxlifetime", 604800);

if (empty($_SERVER["HTTPS"]) || $_SERVER["HTTPS"] === "off") {
    header(
        "Location: https://" . $_SERVER["HTTP_HOST"] . $_SERVER["REQUEST_URI"],
    );
    exit();
}
if (!isset($_SESSION["login"]) || !isset($_SESSION["name"])) {
    header("Location: login.php");
    exit();
}
?>
