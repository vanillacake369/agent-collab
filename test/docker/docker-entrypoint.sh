#!/bin/bash
# =============================================================================
# Docker Entrypoint for Claude Code Integration Testing
# Handles: plugin install, daemon startup, OAuth authentication, and agent execution
# =============================================================================

set -e

AGENT_NAME="${AGENT_NAME:-Agent}"
AGENT_ROLE="${AGENT_ROLE:-developer}"
AGENT_PROMPT="${AGENT_PROMPT:-}"
TOKEN_FILE="/home/agent/.claude/.oauth_token"
PLUGIN_SRC="/opt/agent-collab-plugin"
PLUGIN_DST="/home/agent/.claude/plugins/local/agent-collab"
SETTINGS_SRC="/opt/claude-settings.json"
SETTINGS_DST="/home/agent/.claude/settings.json"

log() { echo "[$(date '+%H:%M:%S')] $1"; }

# =============================================================================
# Install plugin (handles volume mount overwriting ~/.claude)
# =============================================================================
install_plugin() {
    # Ensure .claude directory exists
    mkdir -p /home/agent/.claude/plugins/local

    # Install plugin if not already installed or outdated
    if [ -d "$PLUGIN_SRC" ]; then
        if [ ! -d "$PLUGIN_DST" ] || [ "$PLUGIN_SRC/hooks/hooks.json" -nt "$PLUGIN_DST/hooks/hooks.json" ]; then
            log "Installing agent-collab plugin..."
            rm -rf "$PLUGIN_DST"
            cp -r "$PLUGIN_SRC" "$PLUGIN_DST"
            chmod +x "$PLUGIN_DST/hooks/"*.mjs 2>/dev/null || true
            log "Plugin installed to $PLUGIN_DST"
        fi
    fi

    # Install settings.json if not exists
    if [ -f "$SETTINGS_SRC" ] && [ ! -f "$SETTINGS_DST" ]; then
        log "Installing Claude settings..."
        cp "$SETTINGS_SRC" "$SETTINGS_DST"
        log "Settings installed to $SETTINGS_DST"
    fi
}

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
            # Use grep to extract only the base64 token (ignore JSON logs)
            agent-collab token show 2>/dev/null | grep -E '^eyJ[A-Za-z0-9_-]+$' > "${TOKEN_FILE}" || true
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
                # Extract only the base64 token line (starts with eyJ)
                INVITE_TOKEN=$(grep -E '^eyJ[A-Za-z0-9_-]+$' "${TOKEN_FILE}" | head -1)
                if [ -n "$INVITE_TOKEN" ]; then
                    log "Found valid token (${#INVITE_TOKEN} chars)"
                    break
                else
                    log "Waiting for valid token format..."
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

    # Priority: 1. Function argument, 2. AGENT_PROMPT env, 3. Role-based default
    if [ -z "$prompt" ] && [ -n "$AGENT_PROMPT" ]; then
        prompt="$AGENT_PROMPT"
    fi

    # Default prompts based on role (fallback)
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
            auth-lib)
                prompt="auth-lib의 JWT 관련 코드를 분석하고, 개선이 필요한 부분을 수정해줘. 완료 후 share_context로 변경 내용을 다른 에이전트에게 공유해줘."
                ;;
            user-service)
                prompt="user-service의 API와 DB 코드를 분석하고, 개선이 필요한 부분을 수정해줘. 먼저 search_similar로 auth-lib 관련 컨텍스트를 확인하고, 완료 후 share_context로 공유해줘."
                ;;
            api-gateway)
                prompt="api-gateway의 라우팅 코드를 분석하고, 개선이 필요한 부분을 수정해줘. 먼저 search_similar로 관련 컨텍스트를 확인하고, 완료 후 share_context로 공유해줘."
                ;;
            observer)
                prompt="get_events를 호출해서 클러스터의 최근 이벤트를 확인하고 요약해줘."
                ;;
            *)
                prompt="프로젝트 구조를 분석하고 개선점을 제안해줘. 분석 결과를 share_context로 다른 에이전트에게 공유해줘."
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
        # Install plugin, start daemon, initialize cluster, and keep container running
        install_plugin
        start_daemon
        load_token
        log "Container ready. Use 'docker exec' to run commands."
        log "Plugin installed: $PLUGIN_DST"
        tail -f /dev/null
        ;;

    setup)
        # Setup authentication only
        install_plugin
        load_token
        setup_auth
        ;;

    run)
        # Run agent task
        install_plugin
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
        install_plugin
        start_daemon
        exec /bin/bash
        ;;

    claude)
        # Run claude directly
        install_plugin
        start_daemon
        shift
        exec claude "$@"
        ;;

    *)
        # Pass through to claude
        install_plugin
        start_daemon
        exec claude "$@"
        ;;
esac
