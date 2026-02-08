#!/bin/sh
# Pre-removal script for agent-collab

set -e

# Stop daemon if running
if agent-collab daemon status >/dev/null 2>&1; then
    echo "Stopping agent-collab daemon..."
    agent-collab daemon stop || true
fi

echo "agent-collab will be removed."
echo "Note: Data in ~/.agent-collab will be preserved."
