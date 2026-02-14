#!/bin/bash
# Interest-based Event Routing Verification Script
# This script verifies that events are correctly routed based on Interest patterns

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "============================================"
echo "  Interest-based Routing Verification"
echo "============================================"
echo ""

# Test 1: Alice modifies auth-lib/jwt.go
# Expected: Charlie receives (jwt.go in interests), Bob doesn't (jwt.go not in interests)
test_jwt_change() {
    echo "[Test 1] Alice modifies auth-lib/jwt.go"
    echo "  Expected: Charlie YES, Bob NO"

    # Simulate Alice sharing context for jwt.go change
    docker exec multi-alice agent-collab mcp call share_context '{
        "file_path": "auth-lib/jwt.go",
        "content": "TEST: Added RS256 support to ValidateToken"
    }' > /dev/null 2>&1

    sleep 2

    # Check Bob's events (should NOT have jwt.go event)
    bob_events=$(docker exec multi-bob agent-collab mcp call get_events '{"limit": 10}' 2>/dev/null || echo "")
    if echo "$bob_events" | grep -q "auth-lib/jwt.go"; then
        echo -e "  Bob: ${RED}FAIL${NC} - Received jwt.go event (should not)"
        return 1
    else
        echo -e "  Bob: ${GREEN}PASS${NC} - Did not receive jwt.go event"
    fi

    # Check Charlie's events (should have jwt.go event)
    charlie_events=$(docker exec multi-charlie agent-collab mcp call get_events '{"limit": 10}' 2>/dev/null || echo "")
    if echo "$charlie_events" | grep -q "auth-lib/jwt.go"; then
        echo -e "  Charlie: ${GREEN}PASS${NC} - Received jwt.go event"
    else
        echo -e "  Charlie: ${RED}FAIL${NC} - Did not receive jwt.go event (should have)"
        return 1
    fi

    echo ""
    return 0
}

# Test 2: Alice modifies auth-lib/token.go
# Expected: Bob receives (token.go in interests), Charlie doesn't (token.go not in interests)
test_token_change() {
    echo "[Test 2] Alice modifies auth-lib/token.go"
    echo "  Expected: Bob YES, Charlie NO"

    # Simulate Alice sharing context for token.go change
    docker exec multi-alice agent-collab mcp call share_context '{
        "file_path": "auth-lib/token.go",
        "content": "TEST: Added SessionID field to TokenClaims"
    }' > /dev/null 2>&1

    sleep 2

    # Check Bob's events (should have token.go event)
    bob_events=$(docker exec multi-bob agent-collab mcp call get_events '{"limit": 10}' 2>/dev/null || echo "")
    if echo "$bob_events" | grep -q "auth-lib/token.go"; then
        echo -e "  Bob: ${GREEN}PASS${NC} - Received token.go event"
    else
        echo -e "  Bob: ${RED}FAIL${NC} - Did not receive token.go event (should have)"
        return 1
    fi

    # Check Charlie's events (should NOT have token.go event)
    charlie_events=$(docker exec multi-charlie agent-collab mcp call get_events '{"limit": 10}' 2>/dev/null || echo "")
    if echo "$charlie_events" | grep -q "auth-lib/token.go"; then
        echo -e "  Charlie: ${RED}FAIL${NC} - Received token.go event (should not)"
        return 1
    else
        echo -e "  Charlie: ${GREEN}PASS${NC} - Did not receive token.go event"
    fi

    echo ""
    return 0
}

# Test 3: Bob modifies user-service/api/handler.go
# Expected: Charlie receives (api/* in interests), Alice doesn't (no user-service interest)
test_api_change() {
    echo "[Test 3] Bob modifies user-service/api/handler.go"
    echo "  Expected: Charlie YES, Alice NO"

    # Simulate Bob sharing context for handler.go change
    docker exec multi-bob agent-collab mcp call share_context '{
        "file_path": "user-service/api/handler.go",
        "content": "TEST: Added new endpoint for user profile"
    }' > /dev/null 2>&1

    sleep 2

    # Check Alice's events (should NOT have handler.go event)
    alice_events=$(docker exec multi-alice agent-collab mcp call get_events '{"limit": 10}' 2>/dev/null || echo "")
    if echo "$alice_events" | grep -q "user-service/api/handler.go"; then
        echo -e "  Alice: ${RED}FAIL${NC} - Received handler.go event (should not)"
        return 1
    else
        echo -e "  Alice: ${GREEN}PASS${NC} - Did not receive handler.go event"
    fi

    # Check Charlie's events (should have handler.go event)
    charlie_events=$(docker exec multi-charlie agent-collab mcp call get_events '{"limit": 10}' 2>/dev/null || echo "")
    if echo "$charlie_events" | grep -q "user-service/api/handler.go"; then
        echo -e "  Charlie: ${GREEN}PASS${NC} - Received handler.go event"
    else
        echo -e "  Charlie: ${RED}FAIL${NC} - Did not receive handler.go event (should have)"
        return 1
    fi

    echo ""
    return 0
}

# Test 4: Monitor receives all events
test_monitor() {
    echo "[Test 4] Monitor receives all events"
    echo "  Expected: All events visible"

    # Check monitor logs for all three events
    monitor_logs=$(docker compose -f docker-compose.multi.yml logs monitor 2>/dev/null || echo "")

    events_found=0
    if echo "$monitor_logs" | grep -q "auth-lib/jwt.go" || echo "$monitor_logs" | grep -q "jwt"; then
        ((events_found++))
    fi
    if echo "$monitor_logs" | grep -q "auth-lib/token.go" || echo "$monitor_logs" | grep -q "token"; then
        ((events_found++))
    fi
    if echo "$monitor_logs" | grep -q "user-service/api/handler.go" || echo "$monitor_logs" | grep -q "handler"; then
        ((events_found++))
    fi

    if [ $events_found -ge 2 ]; then
        echo -e "  Monitor: ${GREEN}PASS${NC} - Received $events_found/3 events"
    else
        echo -e "  Monitor: ${YELLOW}WARN${NC} - Only received $events_found/3 events"
    fi

    echo ""
    return 0
}

# Run all tests
main() {
    passed=0
    failed=0

    if test_jwt_change; then ((passed++)); else ((failed++)); fi
    if test_token_change; then ((passed++)); else ((failed++)); fi
    if test_api_change; then ((passed++)); else ((failed++)); fi
    test_monitor  # Info only, doesn't count

    echo "============================================"
    echo "  Results: $passed passed, $failed failed"
    echo "============================================"

    if [ $failed -eq 0 ]; then
        echo -e "${GREEN}All Interest routing tests passed!${NC}"
        exit 0
    else
        echo -e "${RED}Some tests failed.${NC}"
        exit 1
    fi
}

# Check if containers are running
if ! docker ps | grep -q "multi-alice"; then
    echo "Error: Multi-project cluster not running."
    echo "Run: make multi-up"
    exit 1
fi

main
