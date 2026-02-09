#!/bin/bash
# run-all.sh - 전체 테스트 실행
set -euo pipefail

# 색상 정의
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_header() { echo -e "\n${CYAN}========================================${NC}"; echo -e "${CYAN}  $1${NC}"; echo -e "${CYAN}========================================${NC}\n"; }

# 설정
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESULTS_DIR="${SCRIPT_DIR}/../results"
mkdir -p "$RESULTS_DIR"

# 결과 추적
declare -A PHASE_RESULTS
OVERALL_START=$(date +%s)

# 단계 실행
run_phase() {
    local phase_name=$1
    local script_name=$2
    local required=${3:-true}

    log_header "$phase_name"

    local start_time
    start_time=$(date +%s)

    if [[ -x "${SCRIPT_DIR}/${script_name}" ]]; then
        if "${SCRIPT_DIR}/${script_name}"; then
            local end_time
            end_time=$(date +%s)
            local duration=$((end_time - start_time))

            log_info "$phase_name 완료 (${duration}초)"
            PHASE_RESULTS["$phase_name"]="PASS"
            return 0
        else
            local end_time
            end_time=$(date +%s)
            local duration=$((end_time - start_time))

            if [[ "$required" == "true" ]]; then
                log_error "$phase_name 실패 (${duration}초)"
                PHASE_RESULTS["$phase_name"]="FAIL"
                return 1
            else
                log_warn "$phase_name 부분 실패 (${duration}초)"
                PHASE_RESULTS["$phase_name"]="PARTIAL"
                return 0
            fi
        fi
    else
        log_error "스크립트 없음: ${script_name}"
        PHASE_RESULTS["$phase_name"]="SKIP"
        return 1
    fi
}

# 최종 리포트 생성
generate_report() {
    local report_file="${RESULTS_DIR}/summary-$(date +%Y%m%d-%H%M%S).json"
    local overall_end
    overall_end=$(date +%s)
    local total_duration=$((overall_end - OVERALL_START))

    local overall="PASS"
    local pass_count=0
    local fail_count=0
    local partial_count=0

    for phase in "${!PHASE_RESULTS[@]}"; do
        case "${PHASE_RESULTS[$phase]}" in
            PASS) ((pass_count++)) ;;
            FAIL) ((fail_count++)); overall="FAIL" ;;
            PARTIAL) ((partial_count++)) ;;
        esac
    done

    if [[ "$overall" != "FAIL" && $partial_count -gt 0 ]]; then
        overall="PARTIAL"
    fi

    cat > "$report_file" << EOF
{
  "test_suite": "multipass_context_sharing",
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "overall": "$overall",
  "total_duration_sec": $total_duration,
  "summary": {
    "passed": $pass_count,
    "failed": $fail_count,
    "partial": $partial_count
  },
  "phases": {
    "setup": "${PHASE_RESULTS[Setup]:-SKIP}",
    "cluster_init": "${PHASE_RESULTS[Cluster Init]:-SKIP}",
    "lock_test": "${PHASE_RESULTS[Lock Test]:-SKIP}",
    "context_test": "${PHASE_RESULTS[Context Test]:-SKIP}"
  }
}
EOF

    log_header "테스트 결과 요약"
    cat "$report_file"

    echo ""
    echo "============================================"
    echo "  전체 결과: $overall"
    echo "  소요 시간: ${total_duration}초"
    echo "  통과: $pass_count, 실패: $fail_count, 부분: $partial_count"
    echo "============================================"
    echo ""
    echo "상세 결과: $RESULTS_DIR"

    return 0
}

# 사용법 출력
usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --skip-setup    기존 VM 사용 (setup 스킵)"
    echo "  --skip-cleanup  테스트 후 VM 유지"
    echo "  --tier1         Tier 1 테스트만 (cluster, lock)"
    echo "  --tier2         Tier 2 테스트까지 (+ context)"
    echo "  -h, --help      도움말 출력"
    echo ""
}

# 메인 실행
main() {
    local skip_setup=false
    local skip_cleanup=false
    local tier=2

    # 옵션 파싱
    while [[ $# -gt 0 ]]; do
        case $1 in
            --skip-setup) skip_setup=true; shift ;;
            --skip-cleanup) skip_cleanup=true; shift ;;
            --tier1) tier=1; shift ;;
            --tier2) tier=2; shift ;;
            -h|--help) usage; exit 0 ;;
            *) log_error "알 수 없는 옵션: $1"; usage; exit 1 ;;
        esac
    done

    log_header "Multipass 컨텍스트 공유 테스트"
    log_info "시작 시간: $(date)"
    log_info "테스트 Tier: $tier"

    # Phase A: Setup
    if [[ "$skip_setup" == "false" ]]; then
        if ! run_phase "Setup" "setup.sh" true; then
            log_error "Setup 실패 - 테스트 중단"
            generate_report
            exit 1
        fi
    else
        log_warn "Setup 스킵됨"
        PHASE_RESULTS["Setup"]="SKIP"
    fi

    # Phase B: Cluster Init
    if ! run_phase "Cluster Init" "cluster-init.sh" true; then
        log_error "Cluster Init 실패 - 테스트 중단"
        generate_report
        [[ "$skip_cleanup" == "false" ]] && "${SCRIPT_DIR}/cleanup.sh" || true
        exit 1
    fi

    # Phase C: Lock Test (Tier 1)
    run_phase "Lock Test" "test-lock.sh" false

    # Phase D: Context Test (Tier 2)
    if [[ $tier -ge 2 ]]; then
        run_phase "Context Test" "test-context.sh" false
    fi

    # 리포트 생성
    generate_report

    # 정리
    if [[ "$skip_cleanup" == "false" ]]; then
        log_header "Cleanup"
        "${SCRIPT_DIR}/cleanup.sh" || true
    else
        log_warn "Cleanup 스킵됨 - VM 유지"
    fi

    log_info "테스트 완료: $(date)"
}

main "$@"
