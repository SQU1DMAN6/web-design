#!/bin/bash

set -e

ssh root@129.150.63.22 <<'EOF'
echo "Changing directory..."
cd /var/www/ink*/data/www/*/*

pwd

echo "Pulling..."
git pull

echo "Changing ownership..."
chown -R inkdrop-machine:inkdrop-machine .

echo "Running make..."
cd inkdrop
timeout -s INT 5s make update

echo "..."
EOF

echo "."