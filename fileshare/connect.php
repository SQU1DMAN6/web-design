<?php
// https://dash.filess.io/#/register
$host = "1g44z.h.filess.io";
$dbname = "fileshare_describeno";
$user = "fileshare_describeno";
$password = "9bdc9bd9e9334eb6703e93404c2983d6bcb522f5";
$port = "5434";

//Login to the server https://www.phpmyadmin.co/
//https://www.freemysqlhosting.net/account/

//https://www.php.net/manual/en/function.pg-connect.php
// Establish connection
$db_handle = pg_connect("host=$host dbname=$dbname user=$user password=$password port=$port");

// Check for connection
if ($db_handle) {
    echo "Connected successfully<br><br>";
} else {
    echo "Connection failed.";
}

?>