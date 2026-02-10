#!/bin/sh
# Post-installation script for agent-collab

set -e

# Create data directory if it doesn't exist
if [ ! -d "$HOME/.agent-collab" ]; then
    mkdir -p "$HOME/.agent-collab"
fi

echo "agent-collab installed successfully!"
echo ""

# Check for WireGuard
if command -v wg >/dev/null 2>&1; then
    echo "✓ WireGuard detected - VPN features available"
else
    echo "! WireGuard not found - VPN features disabled"
    echo "  To enable VPN features, install wireguard-tools:"
    echo "    Ubuntu/Debian: sudo apt install wireguard-tools"
    echo "    Fedora/RHEL:   sudo dnf install wireguard-tools"
    echo "    Arch Linux:    sudo pacman -S wireguard-tools"
    echo ""
fi

echo ""
echo "Quick start:"
echo "  agent-collab daemon start    # Start background daemon"
echo "  agent-collab init my-project # Initialize a new cluster"
echo "  agent-collab status          # Check status"
echo ""
echo "For MCP integration with Claude Code:"
echo "  claude mcp add agent-collab -- agent-collab mcp serve"
echo ""
