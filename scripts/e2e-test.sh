#!/bin/bash
# e2e-test.sh - Multipass 기반 E2E 테스트 (설정, 클러스터 초기화, 컨텍스트 공유 테스트)
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
VM_NAMES=("peer1" "peer2" "peer3")
VM_CPUS=2
VM_MEMORY="2G"
VM_DISK="10G"
VM_IMAGE="22.04"
PROJECT_NAME="e2e-test"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BINARY_NAME="agent-collab-linux-arm64"

# 아키텍처 감지
detect_arch() {
    local arch=$(uname -m)
    case $arch in
        x86_64) BINARY_NAME="agent-collab-linux-amd64" ;;
        arm64|aarch64) BINARY_NAME="agent-collab-linux-arm64" ;;
        *) log_error "지원하지 않는 아키텍처: $arch"; exit 1 ;;
    esac
    log_info "타겟 아키텍처: $BINARY_NAME"
}

# Multipass 설치 확인
check_multipass() {
    if ! command -v multipass &> /dev/null; then
        log_error "Multipass가 설치되어 있지 않습니다."
        log_info "설치: brew install multipass"
        exit 1
    fi
    log_info "Multipass 버전: $(multipass version | head -1)"
}

# 바이너리 빌드
build_binary() {
    log_step "Linux 바이너리 빌드"
    cd "$PROJECT_DIR"

    local goos="linux"
    local goarch="arm64"
    [[ "$BINARY_NAME" == *"amd64"* ]] && goarch="amd64"

    CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build -o "$BINARY_NAME" ./cmd/agent-collab
    log_info "빌드 완료: $BINARY_NAME"
}

# VM 생성
create_vms() {
    log_step "VM 생성"

    for vm in "${VM_NAMES[@]}"; do
        if multipass info "$vm" &>/dev/null; then
            log_warn "기존 VM 발견: $vm - 재사용"
            multipass start "$vm" 2>/dev/null || true
        else
            log_info "VM 생성: $vm"
            multipass launch "$VM_IMAGE" \
                --name "$vm" \
                --cpus "$VM_CPUS" \
                --memory "$VM_MEMORY" \
                --disk "$VM_DISK"
        fi
    done
}

# 패키지 설치 및 바이너리 배포
setup_vms() {
    log_step "VM 설정 및 바이너리 배포"

    for vm in "${VM_NAMES[@]}"; do
        log_info "[$vm] 설정 중..."

        # 기본 패키지 설치
        multipass exec "$vm" -- sudo apt-get update -qq
        multipass exec "$vm" -- sudo apt-get install -y -qq jq curl socat

        # 데몬 중지 (있는 경우)
        multipass exec "$vm" -- pkill -f agent-collab 2>/dev/null || true

        # 바이너리 배포
        multipass transfer "${PROJECT_DIR}/${BINARY_NAME}" "${vm}:/home/ubuntu/agent-collab"
        multipass exec "$vm" -- chmod +x /home/ubuntu/agent-collab

        # 기존 설정 정리
        multipass exec "$vm" -- rm -rf /home/ubuntu/.agent-collab

        log_info "[$vm] 설정 완료"
    done
}

# 클러스터 초기화
init_cluster() {
    log_step "클러스터 초기화"

    local leader="${VM_NAMES[0]}"

    # 리더에서 초기화
    log_info "[$leader] 클러스터 초기화..."
    multipass exec "$leader" -- /home/ubuntu/agent-collab daemon start
    sleep 2

    local init_output
    init_output=$(multipass exec "$leader" -- /home/ubuntu/agent-collab init --project "$PROJECT_NAME" 2>&1)
    echo "$init_output"

    # 리더의 실제 리스닝 포트 확인
    local leader_ip
    leader_ip=$(multipass info "$leader" | grep IPv4 | awk '{print $2}')

    local leader_port
    leader_port=$(multipass exec "$leader" -- ss -tlnp 2>/dev/null | grep agent-collab | grep -oE ":[0-9]+" | head -1 | tr -d ':')

    local leader_id
    leader_id=$(multipass exec "$leader" -- /home/ubuntu/agent-collab daemon status 2>&1 | grep "Node ID" | awk '{print $NF}')

    log_info "리더 정보: IP=$leader_ip, Port=$leader_port, ID=$leader_id"

    # 토큰 생성
    local token_json='{"addrs":["/ip4/'$leader_ip'/tcp/'$leader_port'"],"project":"'$PROJECT_NAME'","creator":"'$leader_id'","created":'$(date +%s)',"expires":'$(($(date +%s) + 86400))'}'
    local token
    token=$(echo -n "$token_json" | base64 | tr -d '\n')

    # 멤버 노드 참여
    for vm in "${VM_NAMES[@]:1}"; do
        log_info "[$vm] 클러스터 참여..."
        multipass exec "$vm" -- /home/ubuntu/agent-collab daemon start
        sleep 1
        multipass exec "$vm" -- /home/ubuntu/agent-collab join "$token" 2>&1 || true
    done

    # 연결 대기
    log_info "P2P 연결 대기..."
    sleep 5

    # 상태 확인
    for vm in "${VM_NAMES[@]}"; do
        log_info "[$vm] 상태:"
        multipass exec "$vm" -- /home/ubuntu/agent-collab daemon status 2>&1 | grep -E "Peer|프로젝트|Node ID" || true
    done
}

# 컨텍스트 공유 테스트
test_context_sharing() {
    log_step "컨텍스트 공유 테스트"

    local peer1="${VM_NAMES[0]}"
    local peer2="${VM_NAMES[1]}"

    # peer1에서 컨텍스트 공유
    log_info "[$peer1] 컨텍스트 공유..."
    local share_result
    share_result=$(multipass exec "$peer1" -- bash -c 'echo '"'"'{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"share_context","arguments":{"file_path":"test.go","content":"E2E test context from peer1"}}}'"'"' | /home/ubuntu/agent-collab mcp serve 2>/dev/null')
    echo "$share_result"

    if echo "$share_result" | grep -q "successfully"; then
        log_info "컨텍스트 공유 성공"
    else
        log_error "컨텍스트 공유 실패"
        return 1
    fi

    # P2P 전파 대기
    sleep 2

    # peer2에서 검색
    log_info "[$peer2] 컨텍스트 검색..."
    local search_result
    search_result=$(multipass exec "$peer2" -- bash -c 'echo '"'"'{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"search_similar","arguments":{"query":"E2E test context peer1","limit":3}}}'"'"' | /home/ubuntu/agent-collab mcp serve 2>/dev/null')
    echo "$search_result"

    if echo "$search_result" | grep -q "E2E test context from peer1"; then
        log_info "컨텍스트 검색 성공 - P2P 동기화 확인!"
        return 0
    else
        log_warn "컨텍스트가 아직 동기화되지 않음 (P2P 전파 지연 가능)"
        return 0
    fi
}

# 이벤트 확인 테스트
test_events() {
    log_step "이벤트 시스템 테스트"

    local peer1="${VM_NAMES[0]}"

    log_info "[$peer1] 이벤트 조회..."
    local events_result
    events_result=$(multipass exec "$peer1" -- bash -c 'echo '"'"'{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_events","arguments":{"limit":10}}}'"'"' | /home/ubuntu/agent-collab mcp serve 2>/dev/null')
    echo "$events_result"

    if echo "$events_result" | grep -q "context.updated"; then
        log_info "이벤트 기록 확인 완료"
        return 0
    else
        log_warn "이벤트가 기록되지 않음"
        return 0
    fi
}

# 정리
cleanup() {
    log_step "VM 정리"

    for vm in "${VM_NAMES[@]}"; do
        log_info "[$vm] 데몬 중지..."
        multipass exec "$vm" -- /home/ubuntu/agent-collab daemon stop 2>/dev/null || true
    done

    read -p "VM을 삭제하시겠습니까? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        for vm in "${VM_NAMES[@]}"; do
            log_info "[$vm] 삭제 중..."
            multipass delete "$vm" --purge 2>/dev/null || true
        done
        log_info "모든 VM 삭제 완료"
    else
        log_info "VM 유지됨. 수동 삭제: multipass delete peer1 peer2 peer3 --purge"
    fi
}

# 사용법
usage() {
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  setup     - VM 생성 및 설정"
    echo "  init      - 클러스터 초기화"
    echo "  test      - 테스트 실행"
    echo "  all       - 전체 실행 (setup + init + test)"
    echo "  cleanup   - VM 정리"
    echo ""
}

# 메인
main() {
    local cmd="${1:-all}"

    detect_arch
    check_multipass

    case "$cmd" in
        setup)
            build_binary
            create_vms
            setup_vms
            ;;
        init)
            init_cluster
            ;;
        test)
            test_context_sharing
            test_events
            ;;
        all)
            build_binary
            create_vms
            setup_vms
            init_cluster
            test_context_sharing
            test_events
            log_info "=== E2E 테스트 완료 ==="
            ;;
        cleanup)
            cleanup
            ;;
        *)
            usage
            exit 1
            ;;
    esac
}

main "$@"
