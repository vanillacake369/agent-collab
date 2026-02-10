#!/bin/bash
# Test script for check_cohesion functionality

set -e

echo "=== agent-collab cohesion check test ==="

# Build the binary
echo "Building agent-collab..."
cd "$(dirname "$0")/.."
go build -o agent-collab ./src

# Start daemon in background
echo "Starting daemon..."
./agent-collab daemon start &
DAEMON_PID=$!
sleep 2

# Initialize a test cluster
echo "Initializing test cluster..."
./agent-collab init test-cohesion-project

# Test 1: Share initial context
echo ""
echo "=== Test 1: Share initial context ==="
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"share_context","arguments":{"file_path":"auth/handler.go","content":"Implemented JWT-based stateless authentication with 24h token expiry"}}}' | ./agent-collab mcp serve 2>/dev/null

# Test 2: Check cohesion - should be cohesive
echo ""
echo "=== Test 2: Cohesive intention check ==="
echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"check_cohesion","arguments":{"type":"before","intention":"Add JWT refresh token support"}}}' | ./agent-collab mcp serve 2>/dev/null

# Test 3: Check cohesion - should detect conflict
echo ""
echo "=== Test 3: Conflicting intention check ==="
echo '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"check_cohesion","arguments":{"type":"before","intention":"Switch to session-based authentication instead of JWT"}}}' | ./agent-collab mcp serve 2>/dev/null

# Test 4: After check - cohesive
echo ""
echo "=== Test 4: Cohesive result check ==="
echo '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"check_cohesion","arguments":{"type":"after","result":"Added JWT token refresh endpoint"}}}' | ./agent-collab mcp serve 2>/dev/null

# Test 5: After check - conflict
echo ""
echo "=== Test 5: Conflicting result check ==="
echo '{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"check_cohesion","arguments":{"type":"after","result":"Replaced JWT authentication with session cookies"}}}' | ./agent-collab mcp serve 2>/dev/null

# Cleanup
echo ""
echo "=== Cleanup ==="
./agent-collab daemon stop
wait $DAEMON_PID 2>/dev/null || true

echo ""
echo "=== Tests completed ==="
