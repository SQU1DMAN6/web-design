#!/bin/bash

# FtR Deployment Script — v3.3
# Builds, packages, and deploys the FtR manager package to JFtR/ftr-manager.
# No "set -e": errors are handled gracefully with informative fallbacks.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# --- Detect Go environment ---
GOOS="$(go env GOOS 2>/dev/null || echo "")"
GOARCH="$(go env GOARCH 2>/dev/null || echo "")"

# Map GOARCH to FtR-friendly arch names
case "$GOARCH" in
    amd64)   FRIENDLY_ARCH="x64" ;;
    arm64)   FRIENDLY_ARCH="arm64" ;;
    aarch64) FRIENDLY_ARCH="arm64" ;;
    386)     FRIENDLY_ARCH="x86" ;;
    *)       FRIENDLY_ARCH="$GOARCH" ;;
esac

ARCH_DIR="$SCRIPT_DIR/BUILD/$GOOS-$FRIENDLY_ARCH"
BINARY_NAME="ftr"
BINARY_PATH="$ARCH_DIR/$BINARY_NAME"

echo "=== FtR Deployment v3.3 ==="
echo "Detected OS:    $GOOS"
echo "Detected Arch:  $GOARCH ($FRIENDLY_ARCH)"


# --- Check for Go compiler ---
if ! command -v go >/dev/null 2>&1; then
    echo "Warning: Go compiler not found."
    echo "Searching for a pre-built binary matching $GOOS/$FRIENDLY_ARCH..."

    FOUND=""
    for candidate in "$ARCH_DIR/$BINARY_NAME" "$SCRIPT_DIR/$BINARY_NAME"; do
        if [ -f "$candidate" ]; then
            echo "  Found candidate: $candidate"
            if command -v file >/dev/null 2>&1; then
                FILE_INFO=$(file -b "$candidate" 2>/dev/null || echo "")
                echo "  File type: $FILE_INFO"
                if echo "$FILE_INFO" | grep -qi "$FRIENDLY_ARCH\|$GOARCH\|universal\|any"; then
                    echo "  Architecture match confirmed."
                    FOUND="$candidate"
                    break
                else
                    echo "  Warning: arch may not match. Trying next..."
                fi
            else
                if echo "$candidate" | grep -qi "$FRIENDLY_ARCH\|$GOARCH"; then
                    FOUND="$candidate"
                    break
                fi
            fi
        fi
    done

    if [ -z "$FOUND" ]; then
        echo "Error: Go not installed and no matching pre-built binary found."
        echo "Install Go (https://go.dev/dl/) and re-run, or place a binary for"
        echo "$GOOS/$FRIENDLY_ARCH at $ARCH_DIR/$BINARY_NAME"
        exit 1
    fi

    echo "Using pre-built binary: $FOUND"
    cp "$FOUND" "$SCRIPT_DIR/$BINARY_NAME"
    chmod 755 "$SCRIPT_DIR/$BINARY_NAME"
else
    echo "Go compiler found. Building FtR..."
    mkdir -p "$ARCH_DIR"

    if ! go build -o "$BINARY_PATH" . 2>&1; then
        echo "Warning: Standard build failed. Retrying with GOFLAGS=-mod=mod..."
        if ! GOFLAGS=-mod=mod go build -o "$BINARY_PATH" . 2>&1; then
            echo "Error: Build failed even with -mod=mod."
            echo "Workaround: run 'go mod tidy', then re-run deploy.sh."
            echo "Or use a pre-built binary from the BUILD directory."
            exit 1
        fi
    fi
    echo "Build successful: $BINARY_PATH"
    chmod 755 "$BINARY_PATH"
fi

# --- Verify binary ---
if [ ! -f "$BINARY_PATH" ] && [ ! -f "$SCRIPT_DIR/$BINARY_NAME" ]; then
    echo "Error: Binary not found at $BINARY_PATH or $SCRIPT_DIR/$BINARY_NAME"
    exit 1
fi
echo ""

# --- Temp work directory ---
WORK_DIR=$(mktemp -d 2>/dev/null || echo "/tmp/ftr-deploy-$$")
if [ ! -d "$WORK_DIR" ]; then
    WORK_DIR="/tmp/ftr-deploy-$$"
    mkdir -p "$WORK_DIR"
fi
echo "Temporary work directory: $WORK_DIR"

cd "$SCRIPT_DIR"

# --- Step 1: Pack as SQAR (compressed) ---
echo "Step 1: Packing ftr-manager as SQAR (compressed)..."
if go run . pack . -C ftr-manager 2>&1; then
    SQAR_FILE=$(ls ftr-manager*.sqar 2>/dev/null | head -1)
    if [ -n "$SQAR_FILE" ]; then
        mv "$SQAR_FILE" "$WORK_DIR/"
        echo "Uploading to JFtR/ftr-manager..."
        if go run . up "$WORK_DIR/$(basename "$SQAR_FILE")" JFtR/ftr-manager 2>&1; then
            echo "SQAR upload successful."
        else
            echo "Warning: SQAR upload failed. Will try FSDL next."
        fi
        rm -f "$WORK_DIR/$(basename "$SQAR_FILE")"
    fi
else
    echo "Warning: SQAR packing failed. 'sqar' tool may not be installed."
fi

# --- Step 2: Pack as FSDL (fallback) ---
echo ""
echo "Step 2: Packing ftr-manager as FSDL (uncompressed)..."
if go run . pack . -U ftr-manager 2>&1; then
    FSDL_FILE=$(ls ftr-manager*.fsdl 2>/dev/null | head -1)
    if [ -n "$FSDL_FILE" ]; then
        mv "$FSDL_FILE" "$WORK_DIR/"
        echo "Uploading to JFtR/ftr-manager..."
        if go run . up "$WORK_DIR/$(basename "$FSDL_FILE")" JFtR/ftr-manager 2>&1; then
            echo "FSDL upload successful."
        else
            echo "Warning: FSDL upload failed. Check credentials."
        fi
        rm -f "$WORK_DIR/$(basename "$FSDL_FILE")"
    fi
else
    echo "Error: FSDL packing failed. Check source directory."
fi

# --- Step 3: Query ---
echo ""
echo "Step 3: Verifying deployment..."
go run . query JFtR/ftr-manager 2>&1 || echo "Warning: Query returned an error."

# --- Cleanup ---
echo ""
echo "=== Deployment complete ==="
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

echo "Done."