<?php
session_start();

$backend = "http://152.53.54.58:8080"; // AI backend
$path = $_GET["path"] ?? "";
$url = $backend . "/" . ltrim($path,"/");

$ch = curl_init($url);
curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
curl_setopt($ch, CURLOPT_CUSTOMREQUEST, $_SERVER['REQUEST_METHOD']);
curl_setopt($ch, CURLOPT_HTTPHEADER, ["Content-Type: application/json"]);

if($_SERVER['REQUEST_METHOD']==="POST"){
    $input = file_get_contents("php://input");
    curl_setopt($ch, CURLOPT_POSTFIELDS, $input);
}

$response = curl_exec($ch);
$httpcode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
curl_close($ch);

http_response_code($httpcode);
header("Content-Type: application/json");
echo $response;
