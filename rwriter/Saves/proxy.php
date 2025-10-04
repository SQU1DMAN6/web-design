<?php
session_start();
header('X-Accel-Buffering: no');
header('Cache-Control: no-cache, no-store, must-revalidate');
header('Pragma: no-cache');
header('Expires: 0');
header('Connection: keep-alive');

$backend = "http://152.53.36.187:8080"; // AI backend
$path = $_GET["path"] ?? "";
$url = $backend . "/" . ltrim($path, "/");

// Initialize cURL
$ch = curl_init($url);
curl_setopt($ch, CURLOPT_CUSTOMREQUEST, $_SERVER["REQUEST_METHOD"]);
curl_setopt($ch, CURLOPT_HTTPHEADER, [
    "Content-Type: application/json",
    "Connection: keep-alive"
]);
curl_setopt($ch, CURLOPT_TCP_KEEPALIVE, 1);
curl_setopt($ch, CURLOPT_TCP_KEEPIDLE, 30);
curl_setopt($ch, CURLOPT_TCP_KEEPINTVL, 15);
curl_setopt($ch, CURLOPT_FORBID_REUSE, false);
curl_setopt($ch, CURLOPT_FRESH_CONNECT, false);
curl_setopt($ch, CURLOPT_TIMEOUT, 0); // no hard timeout, rely on server
curl_setopt($ch, CURLOPT_RETURNTRANSFER, false); // streaming

if ($_SERVER["REQUEST_METHOD"] === "POST") {
    $input = file_get_contents("php://input");
    curl_setopt($ch, CURLOPT_POSTFIELDS, $input);
}

// Stream the response directly
// Disable output buffering to stream immediately
if (function_exists('apache_setenv')) {
    @apache_setenv('no-gzip', '1');
}
@ini_set('zlib.output_compression', '0');
@ini_set('output_buffering', 'off');
@ini_set('implicit_flush', '1');
for ($i = 0; $i < ob_get_level(); $i++) { @ob_end_flush(); }
ob_implicit_flush(1);

curl_setopt($ch, CURLOPT_WRITEFUNCTION, function ($curl, $data) {
    echo $data;
    if (function_exists('fastcgi_finish_request')) {
        // not finishing request; we just flush buffers quickly
    }
    flush();
    return strlen($data);
});

$httpcode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
curl_exec($ch);

curl_close($ch);
