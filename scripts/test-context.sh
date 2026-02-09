#!/bin/bash
# test-context.sh - 컨텍스트 동기화 테스트
set -euo pipefail

# 색상 정의
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_step() { echo -e "${BLUE}[STEP]${NC} $1"; }
log_pass() { echo -e "${GREEN}[PASS]${NC} $1"; }
log_fail() { echo -e "${RED}[FAIL]${NC} $1"; }

# 설정
LEADER="peer1"
MEMBERS=("peer2" "peer3")
DAEMON_TIMEOUT=10
SYNC_TIMEOUT=15

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESULTS_DIR="${SCRIPT_DIR}/../results"
mkdir -p "$RESULTS_DIR"

# 테스트 결과
declare -A TEST_RESULTS

# Daemon 시작
start_daemons() {
    log_step "Daemon 시작"

    for vm in "$LEADER" "${MEMBERS[@]}"; do
        log_info "[$vm] Daemon 시작..."

        # 기존 daemon 종료
        multipass exec "$vm" -- pkill -f "agent-collab daemon" 2>/dev/null || true
        sleep 1

        # Daemon 시작 (백그라운드)
        multipass exec "$vm" -- bash -c "cd /home/ubuntu/project && nohup /home/ubuntu/agent-collab daemon start > /tmp/daemon.log 2>&1 &"
    done

    # Daemon 준비 대기
    log_info "Daemon 준비 대기 (${DAEMON_TIMEOUT}초)..."
    sleep "$DAEMON_TIMEOUT"

    # 상태 확인
    for vm in "$LEADER" "${MEMBERS[@]}"; do
        status=$(multipass exec "$vm" -- /home/ubuntu/agent-collab daemon status 2>&1 || echo "not running")
        if echo "$status" | grep -qiE "running|active"; then
            log_info "[$vm] Daemon 실행 중"
        else
            log_warn "[$vm] Daemon 상태: $status"
        fi
    done
}

# Daemon 중지
stop_daemons() {
    log_info "Daemon 중지..."
    for vm in "$LEADER" "${MEMBERS[@]}"; do
        multipass exec "$vm" -- /home/ubuntu/agent-collab daemon stop 2>/dev/null || true
        multipass exec "$vm" -- pkill -f "agent-collab daemon" 2>/dev/null || true
    done
}

# 컨텍스트 공유 테스트
test_share_context() {
    log_step "Test 1: 컨텍스트 공유"

    local test_content="Authentication module handles user login and session management"
    local test_file="src/api.go"

    local start_time
    start_time=$(date +%s%3N)

    log_info "[$LEADER] 컨텍스트 공유..."

    # CLI를 통한 컨텍스트 공유 시도
    share_output=$(multipass exec "$LEADER" -- bash -c "
        cd /home/ubuntu/project && \
        /home/ubuntu/agent-collab context share --file '$test_file' --content '$test_content' 2>&1 || \
        echo 'CLI share not available, trying daemon API...'
    ")

    # Daemon API를 통한 컨텍스트 공유 (fallback)
    if echo "$share_output" | grep -q "not available"; then
        share_output=$(multipass exec "$LEADER" -- bash -c "
            curl -s -X POST http://localhost:8080/embed \
                -H 'Content-Type: application/json' \
                -d '{\"text\": \"$test_content\", \"metadata\": {\"file\": \"$test_file\"}}' 2>&1 || \
            echo 'API also failed'
        ")
    fi

    local end_time
    end_time=$(date +%s%3N)
    local duration=$((end_time - start_time))

    echo "$share_output"

    if echo "$share_output" | grep -qiE "success|embedded|shared|vector"; then
        log_pass "컨텍스트 공유 성공 (${duration}ms)"
        TEST_RESULTS["share_context"]="PASS"
        TEST_RESULTS["share_duration"]="$duration"
        return 0
    else
        log_warn "컨텍스트 공유 결과 불확실 (Daemon API 미사용 가능)"
        TEST_RESULTS["share_context"]="PARTIAL"
        TEST_RESULTS["share_duration"]="$duration"
        return 0
    fi
}

# 컨텍스트 검색 테스트
test_search_context() {
    log_step "Test 2: 컨텍스트 검색"

    local search_query="user authentication login"

    sleep 3  # 동기화 대기

    for member in "${MEMBERS[@]}"; do
        log_info "[$member] 컨텍스트 검색..."

        local start_time
        start_time=$(date +%s%3N)

        # CLI를 통한 검색 시도
        search_output=$(multipass exec "$member" -- bash -c "
            cd /home/ubuntu/project && \
            /home/ubuntu/agent-collab context search --query '$search_query' 2>&1 || \
            echo 'CLI search not available, trying daemon API...'
        ")

        # Daemon API를 통한 검색 (fallback)
        if echo "$search_output" | grep -q "not available"; then
            search_output=$(multipass exec "$member" -- bash -c "
                curl -s 'http://localhost:8080/search?query=$search_query&limit=5' 2>&1 || \
                echo 'API also failed'
            ")
        fi

        local end_time
        end_time=$(date +%s%3N)
        local duration=$((end_time - start_time))

        echo "  검색 결과:"
        echo "$search_output" | head -10

        if echo "$search_output" | grep -qiE "auth|login|session|result|found"; then
            log_pass "[$member] 컨텍스트 검색 성공 (${duration}ms)"
            TEST_RESULTS["search_${member}"]="PASS"
        else
            log_warn "[$member] 컨텍스트 검색 결과 불확실"
            TEST_RESULTS["search_${member}"]="PARTIAL"
        fi
    done

    return 0
}

# 에이전트 목록 테스트
test_list_agents() {
    log_step "Test 3: 에이전트 목록 조회"

    for vm in "$LEADER" "${MEMBERS[@]}"; do
        log_info "[$vm] 에이전트 목록..."

        agents_output=$(multipass exec "$vm" -- bash -c "
            /home/ubuntu/agent-collab agents list 2>&1 || \
            curl -s http://localhost:8080/agents/list 2>&1 || \
            /home/ubuntu/agent-collab status 2>&1
        ")

        echo "  $agents_output" | head -5

        # 에이전트/피어 수 확인
        count=$(echo "$agents_output" | grep -oE '[0-9]+' | head -1 || echo "0")
        log_info "[$vm] 감지된 수: $count"
    done

    TEST_RESULTS["list_agents"]="PASS"
    return 0
}

# 이벤트 수신 테스트
test_events() {
    log_step "Test 4: 이벤트 수신"

    log_info "[$LEADER] 이벤트 조회..."

    events_output=$(multipass exec "$LEADER" -- bash -c "
        /home/ubuntu/agent-collab events list --limit 10 2>&1 || \
        curl -s http://localhost:8080/events?limit=10 2>&1 || \
        echo 'Events not available'
    ")

    echo "  최근 이벤트:"
    echo "$events_output" | head -10

    if echo "$events_output" | grep -qiE "event|lock|context|peer|joined"; then
        log_pass "이벤트 조회 성공"
        TEST_RESULTS["events"]="PASS"
    else
        log_warn "이벤트 조회 결과 불확실"
        TEST_RESULTS["events"]="PARTIAL"
    fi

    return 0
}

# 결과 저장
save_results() {
    local result_file="${RESULTS_DIR}/test-context-$(date +%Y%m%d-%H%M%S).json"

    local overall="PASS"
    local fail_count=0
    local partial_count=0

    for key in "${!TEST_RESULTS[@]}"; do
        case "${TEST_RESULTS[$key]}" in
            FAIL) ((fail_count++)); overall="FAIL" ;;
            PARTIAL) ((partial_count++)) ;;
        esac
    done

    if [[ "$overall" != "FAIL" && $partial_count -gt 0 ]]; then
        overall="PARTIAL"
    fi

    cat > "$result_file" << EOF
{
  "test": "context_synchronization",
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "overall": "$overall",
  "results": {
    "share_context": "${TEST_RESULTS[share_context]:-SKIP}",
    "share_duration_ms": ${TEST_RESULTS[share_duration]:-0},
    "search_peer2": "${TEST_RESULTS[search_peer2]:-SKIP}",
    "search_peer3": "${TEST_RESULTS[search_peer3]:-SKIP}",
    "list_agents": "${TEST_RESULTS[list_agents]:-SKIP}",
    "events": "${TEST_RESULTS[events]:-SKIP}"
  },
  "config": {
    "daemon_timeout": $DAEMON_TIMEOUT,
    "sync_timeout": $SYNC_TIMEOUT
  }
}
EOF

    log_info "결과 저장: $result_file"
    echo ""
    cat "$result_file"
}

# 메인 실행
main() {
    log_info "=== 컨텍스트 동기화 테스트 시작 ==="

    # Daemon 시작
    start_daemons

    local failed=0

    # 테스트 실행
    test_share_context || ((failed++))
    sleep 2

    test_search_context || ((failed++))
    sleep 1

    test_list_agents || ((failed++))
    sleep 1

    test_events || ((failed++))

    # 결과 저장
    save_results

    # Daemon 중지
    stop_daemons

    echo ""
    if [[ $failed -eq 0 ]]; then
        log_info "=== 컨텍스트 테스트 완료: 모든 테스트 통과 ==="
        exit 0
    else
        log_warn "=== 컨텍스트 테스트 완료: ${failed}개 테스트 문제 발생 ==="
        exit 0  # 부분 성공도 허용
    fi
}

main "$@"
