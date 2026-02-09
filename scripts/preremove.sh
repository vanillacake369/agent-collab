#!/bin/sh
# Pre-removal script for agent-collab

set -e

# Stop daemon if running
if command -v agent-collab >/dev/null 2>&1; then
    if agent-collab daemon status >/dev/null 2>&1; then
        echo "Stopping agent-collab daemon..."
        agent-collab daemon stop || true
    fi
fi

echo ""
echo "=========================================="
echo "  agent-collab will be removed."
echo "=========================================="
echo ""
echo "Your data in ~/.agent-collab is preserved."
echo ""
echo "To completely remove all data, run:"
echo "  agent-collab data purge"
echo ""
echo "Or manually delete:"
echo "  rm -rf ~/.agent-collab"
echo ""
