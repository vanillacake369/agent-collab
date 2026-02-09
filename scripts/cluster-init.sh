#!/bin/bash
# cluster-init.sh - 클러스터 초기화 및 조인
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

# 설정
LEADER="peer1"
MEMBERS=("peer2" "peer3")
PROJECT_NAME="multipass-test"
TOKEN_FILE="/tmp/agent-collab-token.txt"
TIMEOUT=60

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESULTS_DIR="${SCRIPT_DIR}/../results"
mkdir -p "$RESULTS_DIR"

# VM 상태 확인
check_vms() {
    log_info "VM 상태 확인..."

    for vm in "$LEADER" "${MEMBERS[@]}"; do
        if ! multipass info "$vm" &> /dev/null; then
            log_error "VM '$vm'이 존재하지 않습니다. setup.sh를 먼저 실행하세요."
            exit 1
        fi

        state=$(multipass info "$vm" | grep State | awk '{print $2}')
        if [[ "$state" != "Running" ]]; then
            log_warn "VM '$vm' 시작 중..."
            multipass start "$vm"
        fi
    done

    log_info "모든 VM 실행 중"
}

# 클러스터 초기화 (리더)
init_cluster() {
    log_step "클러스터 초기화 (리더: $LEADER)"

    # 기존 상태 정리
    multipass exec "$LEADER" -- rm -rf /home/ubuntu/.agent-collab 2>/dev/null || true

    # 클러스터 초기화
    log_info "[$LEADER] agent-collab init 실행..."
    init_output=$(multipass exec "$LEADER" -- bash -c "cd /home/ubuntu/project && /home/ubuntu/agent-collab init $PROJECT_NAME 2>&1")

    echo "$init_output"

    # 토큰 추출
    token=$(echo "$init_output" | grep -oE 'agent-collab://[a-zA-Z0-9+/=]+' | head -1 || true)

    if [[ -z "$token" ]]; then
        log_error "초대 토큰을 찾을 수 없습니다."
        log_error "출력: $init_output"
        exit 1
    fi

    echo "$token" > "$TOKEN_FILE"
    log_info "토큰 저장됨: $TOKEN_FILE"
    log_info "토큰: ${token:0:50}..."
}

# 클러스터 참여 (멤버)
join_cluster() {
    local token
    token=$(cat "$TOKEN_FILE")

    for member in "${MEMBERS[@]}"; do
        log_step "클러스터 참여: $member"

        # 기존 상태 정리
        multipass exec "$member" -- rm -rf /home/ubuntu/.agent-collab 2>/dev/null || true

        # 클러스터 참여
        log_info "[$member] agent-collab join 실행..."
        join_output=$(multipass exec "$member" -- bash -c "cd /home/ubuntu/project && /home/ubuntu/agent-collab join '$token' 2>&1") || true

        echo "$join_output"
    done
}

# 피어 연결 대기
wait_for_peers() {
    log_step "피어 연결 대기 (최대 ${TIMEOUT}초)..."

    local start_time
    start_time=$(date +%s)
    local expected_peers=3

    while true; do
        local elapsed
        elapsed=$(($(date +%s) - start_time))

        if [[ $elapsed -gt $TIMEOUT ]]; then
            log_error "타임아웃: ${TIMEOUT}초 내 피어 연결 실패"
            return 1
        fi

        # 리더에서 피어 수 확인
        status_output=$(multipass exec "$LEADER" -- /home/ubuntu/agent-collab status 2>&1 || echo "error")

        # 피어 수 추출 (다양한 포맷 지원)
        peer_count=$(echo "$status_output" | grep -oE 'Peers?:?\s*[0-9]+|[0-9]+ peers?' | grep -oE '[0-9]+' | head -1 || echo "0")

        log_info "연결된 피어: ${peer_count:-0}/${expected_peers} (경과: ${elapsed}초)"

        if [[ "${peer_count:-0}" -ge "$expected_peers" ]]; then
            log_info "모든 피어 연결 완료!"
            return 0
        fi

        sleep 5
    done
}

# 클러스터 상태 확인
verify_cluster() {
    log_step "클러스터 상태 확인"

    local result_file="${RESULTS_DIR}/cluster-init-$(date +%Y%m%d-%H%M%S).json"

    echo "{" > "$result_file"
    echo '  "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",' >> "$result_file"
    echo '  "project": "'$PROJECT_NAME'",' >> "$result_file"
    echo '  "nodes": {' >> "$result_file"

    local first=true
    for vm in "$LEADER" "${MEMBERS[@]}"; do
        if [[ "$first" != "true" ]]; then
            echo "," >> "$result_file"
        fi
        first=false

        log_info "[$vm] 상태 조회..."
        status=$(multipass exec "$vm" -- /home/ubuntu/agent-collab status 2>&1 || echo "error")
        ip=$(multipass info "$vm" | grep IPv4 | awk '{print $2}')

        echo -n '    "'$vm'": {"ip": "'$ip'", "status": ' >> "$result_file"

        if echo "$status" | grep -qiE "running|connected|active"; then
            echo -n '"connected"' >> "$result_file"
        else
            echo -n '"disconnected"' >> "$result_file"
        fi

        echo -n '}' >> "$result_file"

        echo "  [$vm] IP: $ip"
        echo "$status" | head -10
        echo ""
    done

    echo "" >> "$result_file"
    echo "  }" >> "$result_file"
    echo "}" >> "$result_file"

    log_info "결과 저장: $result_file"
}

# 메인 실행
main() {
    log_info "=== 클러스터 초기화 시작 ==="

    check_vms
    init_cluster
    join_cluster

    sleep 3  # 연결 안정화 대기

    if wait_for_peers; then
        verify_cluster
        log_info "=== 클러스터 초기화 성공 ==="
        log_info "다음 단계: ./test-lock.sh"
        exit 0
    else
        log_error "=== 클러스터 초기화 실패 ==="
        verify_cluster
        exit 1
    fi
}

main "$@"
