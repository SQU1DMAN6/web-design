<?php
session_start();
if (empty($_SERVER['HTTPS']) || $_SERVER['HTTPS'] === "off") {
    header("Location: https://" . $_SERVER['HTTP_HOST'] . $_SERVER['REQUEST_URI']);
    exit();
}
if (!isset($_SESSION["login"])) {
    header("Location: login.php");
    exit();
}

if (!isset($_SESSION["name"])) {
    header("location: login.php");
}

?>