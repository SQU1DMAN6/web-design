#!/bin/bash

# FtR Installation Script — v3.3
# Installs FtR to /usr/local/bin/ftr and /usr/local/share/ftr
# No "set -e": errors are handled gracefully with informative fallbacks.

echo "FtR Installation Script v3.3"
echo "You may be prompted for your sudo password for system-wide installation."
echo ""

# --- Detect OS and architecture ---
OS_TYPE="$(uname -s 2>/dev/null | tr '[:upper:]' '[:lower:]' || echo "")"
ARCH_TYPE="$(uname -m 2>/dev/null || echo "")"

case "$ARCH_TYPE" in
    x86_64|amd64) ARCH_TYPE="x64" ;;
    aarch64|arm64) ARCH_TYPE="arm64" ;;
    i386|i686)    ARCH_TYPE="x86" ;;
esac

echo "Detected OS:      $OS_TYPE"
echo "Detected Arch:    $ARCH_TYPE"

# --- Determine script and source directories ---
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILD_DIR="$SCRIPT_DIR/BUILD"
PREBUILT_PATH="$BUILD_DIR/$OS_TYPE-$ARCH_TYPE/ftr"

# --- Create temporary work directory ---
WORK_DIR=$(mktemp -d 2>/dev/null || echo "/tmp/ftr-install-$$")
if [ ! -d "$WORK_DIR" ]; then
    echo "Warning: mktemp failed. Falling back to /tmp/ftr-install-$$"
    WORK_DIR="/tmp/ftr-install-$$"
    mkdir -p "$WORK_DIR"
fi
echo "Created temporary work directory: $WORK_DIR"
echo ""

# --- Create share directory ---
sudo mkdir -p "/usr/local/share/ftr" 2>/dev/null || \
    echo "Warning: /usr/local/share/ftr may already exist or permission denied"

# --- Check for Go compiler and attempt build ---
BUILT_BINARY=""

if command -v go >/dev/null 2>&1; then
    echo "Go compiler found. Building FtR from source..."
    if go build -buildvcs=false -o "$WORK_DIR/ftr" . 2>&1; then
        echo "Build successful."
        BUILT_BINARY="$WORK_DIR/ftr"
        chmod 755 "$BUILT_BINARY"
    else
        echo "Warning: Standard build failed. Retrying with GOFLAGS=-mod=mod..."
        if GOFLAGS=-mod=mod go build -buildvcs=false -o "$WORK_DIR/ftr" . 2>&1; then
            echo "Build successful with -mod=mod."
            BUILT_BINARY="$WORK_DIR/ftr"
            chmod 755 "$BUILT_BINARY"
        else
            echo "Warning: Build from source failed."
            echo "Will attempt to find a pre-built binary matching $OS_TYPE/$ARCH_TYPE."
        fi
    fi
else
    echo "Go compiler not found."
    echo "Will attempt to use a pre-built binary."
fi
echo ""

# --- Find a working binary (built or pre-built) ---
INSTALL_BINARY=""

if [ -n "$BUILT_BINARY" ] && [ -f "$BUILT_BINARY" ]; then
    INSTALL_BINARY="$BUILT_BINARY"
    echo "Using freshly built binary: $INSTALL_BINARY"
else
    echo "Searching for pre-built binaries matching $OS_TYPE/$ARCH_TYPE..."
    for candidate in \
        "$PREBUILT_PATH" \
        "$BUILD_DIR/$OS_TYPE-$ARCH_TYPE/ftr" \
        "$BUILD_DIR/linux-x64/ftr" \
        "$SCRIPT_DIR/BUILD/linux-x64/ftr"; do
        if [ -f "$candidate" ]; then
            echo "  Found: $candidate"
            if command -v file >/dev/null 2>&1; then
                FILE_INFO=$(file -b "$candidate" 2>/dev/null || echo "")
                echo "  File type: $FILE_INFO"
                if echo "$FILE_INFO" | grep -qi "$ARCH_TYPE"; then
                    echo "  Architecture matches $ARCH_TYPE."
                    INSTALL_BINARY="$candidate"
                    break
                else
                    echo "  Warning: Architecture may not match. Skipping."
                fi
            else
                if echo "$candidate" | grep -qi "$ARCH_TYPE"; then
                    INSTALL_BINARY="$candidate"
                    break
                fi
            fi
        fi
    done

    if [ -z "$INSTALL_BINARY" ]; then
        echo ""
        echo "Error: Could not find a compatible FtR binary."
        echo "Options:"
        echo "  1. Install Go (https://go.dev/dl/) and re-run this script."
        echo "  2. Manually build ftr and place it at $PREBUILT_PATH"
        echo "  3. Run 'ftr daemon install' if ftr is already available elsewhere."
        exit 1
    fi
    echo "Using pre-built binary: $INSTALL_BINARY"
fi

# --- Backup existing binary ---
if [ -f "/usr/local/bin/ftr" ]; then
    echo ""
    echo "Existing FtR binary found at /usr/local/bin/ftr."
    read -p "Move it to /usr/local/bin/ftr.old as backup? [y/N] " backup_resp
    case "$backup_resp" in
        [yY][eE][sS]|[yY])
            echo "Backing up existing binary..."
            sudo mv /usr/local/bin/ftr /usr/local/bin/ftr.old 2>/dev/null || \
                echo "Warning: Could not rename existing binary."
            ;;
        *)
            echo "Overwriting existing binary without backup."
            ;;
    esac
fi

# --- Install binary ---
echo ""
echo "Installing binary to /usr/local/bin/ftr..."
sudo cp "$INSTALL_BINARY" /usr/local/share/ftr/ftr 2>/dev/null || \
    echo "Warning: Could not copy to /usr/local/share/ftr/ftr"
sudo cp "$INSTALL_BINARY" /usr/local/bin/ftr 2>/dev/null || \
    echo "Warning: Could not copy to /usr/local/bin/ftr"
sudo chmod 755 /usr/local/bin/ftr 2>/dev/null || true

# --- Install remove.sh ---
echo "Installing remove.sh..."
if [ -f "$SCRIPT_DIR/remove.sh" ]; then
    sudo cp "$SCRIPT_DIR/remove.sh" /usr/local/share/ftr/ 2>/dev/null || \
        echo "Warning: Could not copy remove.sh."
else
    echo "Warning: remove.sh not found. Skipping..."
fi

# --- Verify installation ---
echo ""
echo "Verifying installation..."
if /usr/local/bin/ftr --help >/dev/null 2>&1; then
    /usr/local/bin/ftr version
    echo "Installation verified successfully."
else
    echo "Warning: ftr binary may not be functional."
fi

# --- Cleanup ---
echo ""
echo "=== Installation complete ==="
echo "Temporary work directory: $WORK_DIR"
read -p "Remove temporary work directory ($WORK_DIR)? [y/N] " cleanup_resp
case "$cleanup_resp" in
    [yY][eE][sS]|[yY])
        rm -rf "$WORK_DIR"
        echo "Temporary directory removed."
        ;;
    *)
        echo "Temporary directory kept at: $WORK_DIR"
        ;;
esac

# --- Clean old backup ---
if [ -f "/usr/local/bin/ftr.old" ]; then
    echo ""
    read -p "Remove old binary backup at /usr/local/bin/ftr.old? [y/N] " old_resp
    case "$old_resp" in
        [yY][eE][sS]|[yY])
            sudo rm -f /usr/local/bin/ftr.old 2>/dev/null && echo "Old backup removed."
            ;;
        *)
            echo "Old backup kept at /usr/local/bin/ftr.old"
            ;;
    esac
fi

echo ""
echo "You're all set. Run 'ftr --help' to get started."