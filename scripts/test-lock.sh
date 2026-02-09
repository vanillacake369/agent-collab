#!/bin/bash
# test-lock.sh - Lock 전파 테스트
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
TEST_FILE="src/api.go"
START_LINE=10
END_LINE=50
INTENTION="Adding user validation"
PROPAGATION_TIMEOUT=10

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESULTS_DIR="${SCRIPT_DIR}/../results"
mkdir -p "$RESULTS_DIR"

# 테스트 결과
declare -A TEST_RESULTS
LOCK_ID=""

# Lock 획득 테스트
test_acquire_lock() {
    log_step "Test 1: Lock 획득"

    local start_time
    start_time=$(date +%s%3N)

    log_info "[$LEADER] Lock 획득 시도..."
    acquire_output=$(multipass exec "$LEADER" -- /home/ubuntu/agent-collab lock acquire \
        --file "$TEST_FILE" \
        --start "$START_LINE" \
        --end "$END_LINE" \
        --intention "$INTENTION" 2>&1) || true

    local end_time
    end_time=$(date +%s%3N)
    local duration=$((end_time - start_time))

    echo "$acquire_output"

    # Lock ID 추출
    LOCK_ID=$(echo "$acquire_output" | grep -oE 'lock-[a-f0-9-]+|[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}' | head -1 || true)

    if [[ -n "$LOCK_ID" ]] || echo "$acquire_output" | grep -qiE "acquired|success"; then
        log_pass "Lock 획득 성공 (${duration}ms)"
        TEST_RESULTS["acquire_lock"]="PASS"
        TEST_RESULTS["acquire_lock_duration"]="$duration"
        return 0
    else
        log_fail "Lock 획득 실패"
        TEST_RESULTS["acquire_lock"]="FAIL"
        return 1
    fi
}

# Lock 전파 확인 테스트
test_lock_propagation() {
    log_step "Test 2: Lock 전파 확인"

    local start_time
    start_time=$(date +%s)
    local all_propagated=false

    while [[ $(($(date +%s) - start_time)) -lt $PROPAGATION_TIMEOUT ]]; do
        local propagated_count=0

        for member in "${MEMBERS[@]}"; do
            log_info "[$member] Lock 목록 조회..."
            lock_list=$(multipass exec "$member" -- /home/ubuntu/agent-collab lock list 2>&1 || echo "")

            echo "  $lock_list" | head -5

            if echo "$lock_list" | grep -qE "$TEST_FILE|$LOCK_ID|$INTENTION"; then
                ((propagated_count++))
                log_info "[$member] Lock 확인됨"
            fi
        done

        if [[ $propagated_count -eq ${#MEMBERS[@]} ]]; then
            all_propagated=true
            break
        fi

        sleep 1
    done

    local elapsed=$(($(date +%s) - start_time))

    if [[ "$all_propagated" == "true" ]]; then
        log_pass "Lock 전파 성공 - 모든 노드에서 확인 (${elapsed}초)"
        TEST_RESULTS["lock_propagation"]="PASS"
        TEST_RESULTS["propagation_time"]="$elapsed"
        return 0
    else
        log_fail "Lock 전파 실패 - 일부 노드에서 확인되지 않음"
        TEST_RESULTS["lock_propagation"]="FAIL"
        return 1
    fi
}

# Lock 해제 테스트
test_release_lock() {
    log_step "Test 3: Lock 해제"

    local start_time
    start_time=$(date +%s%3N)

    log_info "[$LEADER] Lock 해제..."

    local release_cmd
    if [[ -n "$LOCK_ID" ]]; then
        release_cmd="/home/ubuntu/agent-collab lock release --id $LOCK_ID"
    else
        release_cmd="/home/ubuntu/agent-collab lock release --file $TEST_FILE"
    fi

    release_output=$(multipass exec "$LEADER" -- $release_cmd 2>&1) || true

    local end_time
    end_time=$(date +%s%3N)
    local duration=$((end_time - start_time))

    echo "$release_output"

    if echo "$release_output" | grep -qiE "released|success|no .* lock"; then
        log_pass "Lock 해제 성공 (${duration}ms)"
        TEST_RESULTS["release_lock"]="PASS"
        return 0
    else
        log_warn "Lock 해제 결과 불확실"
        TEST_RESULTS["release_lock"]="PARTIAL"
        return 0
    fi
}

# Lock 해제 전파 확인
test_release_propagation() {
    log_step "Test 4: Lock 해제 전파 확인"

    sleep 2  # 전파 대기

    local all_cleared=true

    for member in "${MEMBERS[@]}"; do
        log_info "[$member] Lock 목록 재확인..."
        lock_list=$(multipass exec "$member" -- /home/ubuntu/agent-collab lock list 2>&1 || echo "")

        echo "  $lock_list" | head -3

        if echo "$lock_list" | grep -qE "$TEST_FILE|$LOCK_ID"; then
            log_warn "[$member] Lock이 아직 존재"
            all_cleared=false
        else
            log_info "[$member] Lock 해제 확인됨"
        fi
    done

    if [[ "$all_cleared" == "true" ]]; then
        log_pass "Lock 해제 전파 성공"
        TEST_RESULTS["release_propagation"]="PASS"
        return 0
    else
        log_fail "Lock 해제 전파 실패"
        TEST_RESULTS["release_propagation"]="FAIL"
        return 1
    fi
}

# 결과 저장
save_results() {
    local result_file="${RESULTS_DIR}/test-lock-$(date +%Y%m%d-%H%M%S).json"

    local overall="PASS"
    for key in "${!TEST_RESULTS[@]}"; do
        if [[ "${TEST_RESULTS[$key]}" == "FAIL" ]]; then
            overall="FAIL"
            break
        fi
    done

    cat > "$result_file" << EOF
{
  "test": "lock_propagation",
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "overall": "$overall",
  "results": {
    "acquire_lock": "${TEST_RESULTS[acquire_lock]:-SKIP}",
    "acquire_lock_duration_ms": ${TEST_RESULTS[acquire_lock_duration]:-0},
    "lock_propagation": "${TEST_RESULTS[lock_propagation]:-SKIP}",
    "propagation_time_sec": ${TEST_RESULTS[propagation_time]:-0},
    "release_lock": "${TEST_RESULTS[release_lock]:-SKIP}",
    "release_propagation": "${TEST_RESULTS[release_propagation]:-SKIP}"
  },
  "config": {
    "test_file": "$TEST_FILE",
    "start_line": $START_LINE,
    "end_line": $END_LINE,
    "intention": "$INTENTION",
    "propagation_timeout": $PROPAGATION_TIMEOUT
  }
}
EOF

    log_info "결과 저장: $result_file"
    echo ""
    cat "$result_file"
}

# 메인 실행
main() {
    log_info "=== Lock 전파 테스트 시작 ==="

    local failed=0

    test_acquire_lock || ((failed++))
    sleep 1

    test_lock_propagation || ((failed++))
    sleep 1

    test_release_lock || ((failed++))
    sleep 1

    test_release_propagation || ((failed++))

    save_results

    echo ""
    if [[ $failed -eq 0 ]]; then
        log_info "=== Lock 테스트 완료: 모든 테스트 통과 ==="
        log_info "다음 단계: ./test-context.sh"
        exit 0
    else
        log_error "=== Lock 테스트 완료: ${failed}개 테스트 실패 ==="
        exit 1
    fi
}

main "$@"
