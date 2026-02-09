#!/bin/bash
# cleanup.sh - VM 삭제 및 정리
set -euo pipefail

# 색상 정의
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }

# 설정
VM_NAMES=("peer1" "peer2" "peer3")

# VM 삭제
delete_vms() {
    log_info "VM 삭제 중..."

    for vm in "${VM_NAMES[@]}"; do
        if multipass info "$vm" &> /dev/null; then
            log_info "삭제: $vm"
            multipass delete "$vm" 2>/dev/null || true
        else
            log_warn "VM '$vm' 없음"
        fi
    done

    log_info "삭제된 VM 정리..."
    multipass purge 2>/dev/null || true
}

# 임시 파일 정리
cleanup_temp() {
    log_info "임시 파일 정리..."

    rm -f /tmp/agent-collab-token.txt
    rm -f /tmp/agent-collab-*.log
}

# 메인 실행
main() {
    log_info "=== 정리 시작 ==="

    delete_vms
    cleanup_temp

    log_info "=== 정리 완료 ==="

    multipass list 2>/dev/null || true
}

main "$@"
