#!/bin/bash

set -e
echo "You sudo password may be prompted. If so, please provide your sudo password. This is necessary for the FtR installation."
sudo echo "Installing FtR package manager..."

sudo rm -rf /tmp/fsdl/
mkdir -p /tmp/fsdl/
cd /tmp/fsdl/

echo "Fetching source code..."
curl -sSL https://quanthai.net/ftr-manager-3.2.0-all-linux.fsdl -o ftr-manager.fsdl
sudo unzip -qq ftr-manager.fsdl
sudo chown -R $(whoami):$(whoami) /tmp/fsdl

if ! command -v go >/dev/null 2>&1; then
	if [ "$(uname -m)" = "x86_64" ]; then
		echo "Architecture is x86_64"
		echo "Go is not installed, will proceed to use pre-compiled binary."
		./BUILD/linux-x64/ftr get JFtR/ftr-manager
	else
		echo "Architecture is not x86_64. If you want to use FtR, please consider installing Go."
		exit 1
	fi
else
	echo "Go is installed, proceeding to build FtR from source..."
	go run . get JFtR/ftr-manager
fi

echo "FtR package manager has been installed successfully. Use 'ftr' as shell command to use it."
echo "ftr --help"
ftr --help

echo "Removing evidence (A.K.A. Cleaning up)..."
sudo ftr clear
