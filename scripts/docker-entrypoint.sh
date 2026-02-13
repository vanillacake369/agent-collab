#!/bin/bash
# =============================================================================
# Docker Entrypoint for Claude Code Integration Testing
# Handles: daemon startup, OAuth authentication, and agent execution
# =============================================================================

set -e

AGENT_NAME="${AGENT_NAME:-Agent}"
AGENT_ROLE="${AGENT_ROLE:-developer}"
TOKEN_FILE="/home/agent/.claude/.oauth_token"

log() { echo "[$(date '+%H:%M:%S')] $1"; }

# Load saved token if exists
load_token() {
    if [ -f "$TOKEN_FILE" ] && [ -z "$CLAUDE_CODE_OAUTH_TOKEN" ]; then
        export CLAUDE_CODE_OAUTH_TOKEN=$(cat "$TOKEN_FILE")
        log "Loaded OAuth token from saved file"
    fi
}

# Save token for future use
save_token() {
    local token="$1"
    echo "$token" > "$TOKEN_FILE"
    chmod 600 "$TOKEN_FILE"
    log "Token saved to $TOKEN_FILE"
}

# =============================================================================
# Start agent-collab daemon and initialize/join cluster
# =============================================================================
start_daemon() {
    local TOKEN_FILE="/workspace/.collab-token"
    local CONFIG_FILE="/data/config.json"

    # Check if already initialized (config.json exists)
    if [ -f "$CONFIG_FILE" ]; then
        log "Cluster already initialized, starting daemon..."
        agent-collab daemon start 2>&1 || true
        sleep 1
        agent-collab status 2>/dev/null || true
        return 0
    fi

    if [ "${AGENT_COLLAB_BOOTSTRAP}" = "true" ]; then
        # Bootstrap node: initialize (init command starts daemon automatically)
        log "Initializing new cluster as bootstrap node..."

        # Remove old token file to prevent peers from using stale tokens
        rm -f "${TOKEN_FILE}"

        agent-collab init -p "collab-test" 2>&1 | tee /data/init.log

        if [ $? -eq 0 ]; then
            # Wait a bit for daemon to stabilize
            sleep 2

            # Save token to shared workspace for other nodes
            agent-collab token show 2>/dev/null > "${TOKEN_FILE}" || true
            if [ -s "${TOKEN_FILE}" ]; then
                log "Invite token saved: $(cat ${TOKEN_FILE} | head -c 30)..."
            else
                log "WARNING: Failed to get invite token"
            fi
        else
            log "ERROR: Initialization failed"
            return 1
        fi
    else
        # Peer node: wait for token file, join, then start daemon
        log "Waiting for invite token from bootstrap node..."

        # Delete any existing stale token first
        rm -f "${TOKEN_FILE}"

        local INVITE_TOKEN=""
        for i in $(seq 1 90); do
            if [ -s "${TOKEN_FILE}" ]; then
                INVITE_TOKEN=$(cat "${TOKEN_FILE}" | head -1)
                # Validate token looks like base64 (not an error message)
                if echo "$INVITE_TOKEN" | grep -qE '^[A-Za-z0-9_-]+$'; then
                    break
                else
                    log "Invalid token format, waiting..."
                    INVITE_TOKEN=""
                fi
            fi
            sleep 1
        done

        if [ -n "${INVITE_TOKEN}" ]; then
            log "Found token, joining cluster..."
            # join command has built-in retry with exponential backoff
            # and starts daemon automatically after successful join
            agent-collab join "${INVITE_TOKEN}" 2>&1 | tee /data/join.log

            if [ $? -ne 0 ]; then
                log "WARNING: Join failed"
            else
                # Wait a bit for daemon to stabilize
                sleep 2
            fi
        else
            log "ERROR: No invite token found after 90 seconds"
            return 1
        fi
    fi

    # Show cluster status
    agent-collab status 2>/dev/null || true
}

# =============================================================================
# Check Claude authentication
# =============================================================================
check_auth() {
    # Try a simple claude command to check auth
    if claude --version &>/dev/null; then
        # Check if actually authenticated by trying to start a session
        if echo "exit" | timeout 10 claude --print "say ok" &>/dev/null; then
            return 0
        fi
    fi
    return 1
}

# =============================================================================
# Setup Claude authentication
# =============================================================================
setup_auth() {
    log "Claude authentication required"
    log ""
    log "============================================"
    log "  CLAUDE CODE AUTHENTICATION"
    log "============================================"
    log ""
    log "Running: claude setup-token"
    log ""

    # Run setup-token and capture output
    local output
    output=$(claude setup-token 2>&1 | tee /dev/tty)

    # Extract token from output
    local token
    token=$(echo "$output" | grep -o 'sk-ant-oat[a-zA-Z0-9_-]*' | head -1)

    if [ -n "$token" ]; then
        save_token "$token"
        export CLAUDE_CODE_OAUTH_TOKEN="$token"
        log "Token extracted and saved automatically!"
    else
        log ""
        log "Could not auto-extract token. Please enter it manually:"
        read -r token
        if [ -n "$token" ]; then
            save_token "$token"
            export CLAUDE_CODE_OAUTH_TOKEN="$token"
        fi
    fi
}

# =============================================================================
# Run agent task
# =============================================================================
run_agent() {
    local prompt="$1"

    log "Running ${AGENT_NAME} (${AGENT_ROLE})..."

    # Default prompts based on role
    # 플러그인 훅이 Edit/Write 시 자동으로 락을 처리하므로 프롬프트는 단순하게
    if [ -z "$prompt" ]; then
        case "$AGENT_ROLE" in
            authentication)
                prompt="main.go의 initAuth()와 authenticate() 함수를 JWT 토큰 검증 로직으로 구현해줘. 작업 전에 search_similar로 관련 컨텍스트를 확인하고, 완료 후 share_context로 다른 에이전트에게 공유해줘."
                ;;
            database)
                prompt="main.go의 initDB()와 connectDB() 함수를 PostgreSQL 연결 풀로 구현해줘. 먼저 search_similar로 인증 관련 컨텍스트를 확인하고, 완료 후 share_context로 공유해줘."
                ;;
            api)
                prompt="main.go의 setupRoutes()와 handleAPI() 함수를 RESTful API로 구현해줘. 먼저 search_similar로 인증과 DB 컨텍스트를 확인하고, 완료 후 share_context로 공유해줘."
                ;;
            *)
                prompt="프로젝트 구조를 분석하고 개선점을 제안해줘."
                ;;
        esac
    fi

    log "Prompt: $prompt"
    log "---"

    # 플러그인이 자동으로 MCP와 훅을 처리함
    # --dangerously-skip-permissions: 테스트 환경에서 권한 승인 자동화
    claude --print --dangerously-skip-permissions "$prompt"

    log "---"
    log "${AGENT_NAME} task completed."
}

# =============================================================================
# Main
# =============================================================================

case "${1:-idle}" in
    idle)
        # Start daemon, initialize cluster, and keep container running
        start_daemon
        load_token
        log "Container ready. Use 'docker exec' to run commands."
        tail -f /dev/null
        ;;

    setup)
        # Setup authentication only
        load_token
        setup_auth
        ;;

    run)
        # Run agent task
        start_daemon
        load_token

        if ! check_auth; then
            setup_auth
        fi

        shift
        run_agent "$*"
        ;;

    shell)
        # Interactive shell
        start_daemon
        exec /bin/bash
        ;;

    claude)
        # Run claude directly
        start_daemon
        shift
        exec claude "$@"
        ;;

    *)
        # Pass through to claude
        start_daemon
        exec claude "$@"
        ;;
esac
