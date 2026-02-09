#!/bin/bash
# setup.sh - Multipass VM 생성 및 초기 설정
set -euo pipefail

# 색상 정의
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# 설정
VM_NAMES=("peer1" "peer2" "peer3")
VM_CPUS=2
VM_MEMORY="2G"
VM_DISK="10G"
VM_IMAGE="22.04"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BINARY_PATH="${PROJECT_DIR}/../agent-collab/agent-collab-linux"

# 바이너리 빌드 확인
check_binary() {
    if [[ ! -f "$BINARY_PATH" ]]; then
        log_info "Linux 바이너리 빌드 중..."
        cd "${PROJECT_DIR}/../agent-collab"
        GOOS=linux GOARCH=amd64 go build -o agent-collab-linux ./cmd/agent-collab
        log_info "빌드 완료: $BINARY_PATH"
    else
        log_info "기존 바이너리 사용: $BINARY_PATH"
    fi
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

# 기존 VM 정리
cleanup_existing() {
    for vm in "${VM_NAMES[@]}"; do
        if multipass list | grep -q "^${vm}"; then
            log_warn "기존 VM 삭제: $vm"
            multipass delete "$vm" --purge 2>/dev/null || true
        fi
    done
}

# VM 생성
create_vms() {
    log_info "VM 생성 시작..."

    for vm in "${VM_NAMES[@]}"; do
        log_info "생성 중: $vm (CPU: $VM_CPUS, RAM: $VM_MEMORY, Disk: $VM_DISK)"
        multipass launch "$VM_IMAGE" \
            --name "$vm" \
            --cpus "$VM_CPUS" \
            --memory "$VM_MEMORY" \
            --disk "$VM_DISK"
    done

    log_info "모든 VM 생성 완료"
}

# 소프트웨어 설치
install_software() {
    log_info "소프트웨어 설치 중..."

    for vm in "${VM_NAMES[@]}"; do
        log_info "[$vm] 패키지 설치..."
        multipass exec "$vm" -- sudo apt-get update -qq
        multipass exec "$vm" -- sudo apt-get install -y -qq jq curl

        # Node.js 설치 (Claude Code용, 선택적)
        # multipass exec "$vm" -- bash -c "curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -"
        # multipass exec "$vm" -- sudo apt-get install -y nodejs
    done
}

# 바이너리 배포
deploy_binary() {
    log_info "agent-collab 바이너리 배포 중..."

    for vm in "${VM_NAMES[@]}"; do
        log_info "[$vm] 바이너리 전송..."
        multipass transfer "$BINARY_PATH" "${vm}:/home/ubuntu/agent-collab"
        multipass exec "$vm" -- chmod +x /home/ubuntu/agent-collab

        # 버전 확인
        version=$(multipass exec "$vm" -- /home/ubuntu/agent-collab version 2>/dev/null || echo "unknown")
        log_info "[$vm] agent-collab 버전: $version"
    done
}

# 테스트 프로젝트 설정
setup_project() {
    log_info "테스트 프로젝트 설정 중..."

    for vm in "${VM_NAMES[@]}"; do
        log_info "[$vm] 프로젝트 디렉토리 생성..."
        multipass exec "$vm" -- mkdir -p /home/ubuntu/project/src
        multipass exec "$vm" -- mkdir -p /home/ubuntu/test-results

        # 테스트 파일 생성
        multipass exec "$vm" -- bash -c 'cat > /home/ubuntu/project/src/api.go << EOF
package api

// UserService handles user authentication
type UserService struct {
    db *Database
}

// Authenticate validates user credentials
func (s *UserService) Authenticate(username, password string) (*User, error) {
    // TODO: Implement authentication logic
    return nil, nil
}

// CreateUser creates a new user account
func (s *UserService) CreateUser(username, email string) (*User, error) {
    // TODO: Implement user creation
    return nil, nil
}
EOF'
    done
}

# VM 정보 출력
show_info() {
    log_info "=== VM 정보 ==="
    multipass list

    echo ""
    log_info "=== IP 주소 ==="
    for vm in "${VM_NAMES[@]}"; do
        ip=$(multipass info "$vm" | grep IPv4 | awk '{print $2}')
        echo "  $vm: $ip"
    done
}

# 메인 실행
main() {
    log_info "=== Multipass 테스트 환경 설정 시작 ==="

    check_multipass
    check_binary
    cleanup_existing
    create_vms
    install_software
    deploy_binary
    setup_project
    show_info

    log_info "=== 설정 완료 ==="
    log_info "다음 단계: ./cluster-init.sh"
}

main "$@"
