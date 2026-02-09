#!/bin/bash
# agent-collab installer
# Usage: curl -fsSL https://raw.githubusercontent.com/<owner>/agent-collab/main/scripts/install.sh | bash

set -euo pipefail

REPO="agent-collab/agent-collab"  # Update with actual repo
BINARY_NAME="agent-collab"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info() { echo -e "${GREEN}[INFO]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$OS" in
        linux) OS="linux" ;;
        darwin) OS="darwin" ;;
        mingw*|msys*|cygwin*) OS="windows" ;;
        *) error "Unsupported OS: $OS" ;;
    esac

    case "$ARCH" in
        x86_64|amd64) ARCH="amd64" ;;
        arm64|aarch64) ARCH="arm64" ;;
        armv7l) ARCH="armv7" ;;
        *) error "Unsupported architecture: $ARCH" ;;
    esac

    echo "${OS}_${ARCH}"
}

# Get latest version from GitHub
get_latest_version() {
    curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/'
}

# Download and install
install() {
    PLATFORM=$(detect_platform)
    VERSION=${VERSION:-$(get_latest_version)}

    info "Detected platform: $PLATFORM"
    info "Installing version: $VERSION"

    # Construct download URL
    case "$PLATFORM" in
        windows_*)
            ARCHIVE="${BINARY_NAME}_v${VERSION}_${PLATFORM}.zip"
            ;;
        *)
            ARCHIVE="${BINARY_NAME}_v${VERSION}_${PLATFORM}.tar.gz"
            ;;
    esac

    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/v${VERSION}/${ARCHIVE}"

    # Create temp directory
    TMP_DIR=$(mktemp -d)
    trap "rm -rf $TMP_DIR" EXIT

    info "Downloading $DOWNLOAD_URL"
    curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/$ARCHIVE"

    # Extract
    info "Extracting archive..."
    cd "$TMP_DIR"
    case "$ARCHIVE" in
        *.zip) unzip -q "$ARCHIVE" ;;
        *.tar.gz) tar -xzf "$ARCHIVE" ;;
    esac

    # Find binary
    BINARY=$(find . -name "$BINARY_NAME" -type f | head -1)
    if [ -z "$BINARY" ]; then
        error "Binary not found in archive"
    fi

    # Install
    info "Installing to $INSTALL_DIR"
    if [ -w "$INSTALL_DIR" ]; then
        mv "$BINARY" "$INSTALL_DIR/$BINARY_NAME"
        chmod +x "$INSTALL_DIR/$BINARY_NAME"
    else
        sudo mv "$BINARY" "$INSTALL_DIR/$BINARY_NAME"
        sudo chmod +x "$INSTALL_DIR/$BINARY_NAME"
    fi

    info "✓ agent-collab $VERSION installed successfully!"
    echo ""
    echo "Get started:"
    echo "  agent-collab --help"
    echo "  agent-collab daemon start"
}

# Uninstall
uninstall() {
    info "Uninstalling agent-collab..."

    if [ -f "$INSTALL_DIR/$BINARY_NAME" ]; then
        if [ -w "$INSTALL_DIR" ]; then
            rm "$INSTALL_DIR/$BINARY_NAME"
        else
            sudo rm "$INSTALL_DIR/$BINARY_NAME"
        fi
        info "✓ agent-collab uninstalled"
    else
        warn "agent-collab not found in $INSTALL_DIR"
    fi
}

# Main
case "${1:-install}" in
    install) install ;;
    uninstall) uninstall ;;
    *) error "Usage: $0 [install|uninstall]" ;;
esac
