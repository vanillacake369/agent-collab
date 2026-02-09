# Multipass Context Sharing Test

agent-collab의 P2P 컨텍스트 공유 기능을 Multipass VM 환경에서 테스트합니다.

## 개요

이 테스트 스위트는 다음을 검증합니다:

1. **클러스터 형성**: 3개 VM이 libp2p P2P 네트워크로 연결
2. **Lock 전파**: Gossipsub을 통한 의미적 잠금 동기화
3. **컨텍스트 동기화**: 벡터 임베딩 기반 컨텍스트 공유
4. **MCP 통합**: Claude Code와의 MCP 도구 연동 (선택)

## 빠른 시작

### 1. 사전 요구사항

```bash
# Multipass 설치 (macOS)
brew install multipass

# Multipass 확인
multipass version
```

### 2. 전체 테스트 실행

```bash
# 전체 테스트 (setup → test → cleanup)
make multipass-test

# 또는 스크립트 직접 실행
chmod +x scripts/*.sh
./scripts/run-all.sh
```

### 3. 단계별 실행

```bash
# 1. VM 설정
make multipass-setup

# 2. 클러스터 초기화
make multipass-init

# 3. Lock 테스트
make multipass-test-lock

# 4. Context 테스트
make multipass-test-context

# 5. 정리
make multipass-cleanup
```

## 테스트 시나리오

### Tier 1: Core Tests

| 테스트 | 설명 | 성공 기준 |
|-------|------|----------|
| Cluster Formation | 3개 VM P2P 연결 | 60초 내 연결 |
| Lock Acquire | Lock 획득 | 응답 < 2초 |
| Lock Propagation | Lock 전파 | 5초 내 동기화 |
| Lock Release | Lock 해제 | 전체 노드 확인 |

### Tier 2: Advanced Tests

| 테스트 | 설명 | 성공 기준 |
|-------|------|----------|
| Share Context | 컨텍스트 공유 | 임베딩 생성 |
| Search Similar | 유사 검색 | 10초 내 검색 |
| List Agents | 에이전트 목록 | 3개 표시 |
| Get Events | 이벤트 조회 | 이벤트 수신 |

## 실행 옵션

```bash
# Tier 1만 실행
make multipass-test-tier1
./scripts/run-all.sh --tier1

# VM 유지 (디버깅용)
make multipass-test-keep
./scripts/run-all.sh --skip-cleanup

# 기존 VM 재사용
./scripts/run-all.sh --skip-setup
```

## 결과 확인

```bash
# 결과 파일 목록
make multipass-results

# 최신 요약
cat results/summary-*.json | jq .

# VM 상태
make multipass-status
```

### 결과 형식

```json
{
  "test_suite": "multipass_context_sharing",
  "overall": "PASS",
  "total_duration_sec": 180,
  "summary": {
    "passed": 4,
    "failed": 0,
    "partial": 0
  }
}
```

## 트러블슈팅

### VM 생성 실패

```bash
# Multipass 재시작
sudo launchctl stop com.canonical.multipassd
sudo launchctl start com.canonical.multipassd

# 기존 VM 정리
multipass delete --purge peer1 peer2 peer3
```

### 피어 연결 실패

```bash
# VM 네트워크 확인
multipass exec peer1 -- ip addr

# libp2p 로그
multipass exec peer1 -- cat /tmp/daemon.log
```

### Lock 전파 안됨

```bash
# 상태 확인
multipass exec peer1 -- /home/ubuntu/agent-collab status --verbose

# 수동 lock 확인
multipass exec peer2 -- /home/ubuntu/agent-collab lock list
```

## 아키텍처

```
┌─────────────────────────────────────────────┐
│              Host Machine (macOS)            │
│                                              │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐    │
│  │  peer1   │ │  peer2   │ │  peer3   │    │
│  │ (leader) │ │ (member) │ │ (member) │    │
│  │          │ │          │ │          │    │
│  │ agent-   │ │ agent-   │ │ agent-   │    │
│  │ collab   │ │ collab   │ │ collab   │    │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘    │
│       │            │            │           │
│       └────────────┼────────────┘           │
│                    │                        │
│            libp2p Gossipsub                 │
│                                              │
└──────────────────────────────────────────────┘
```

## 관련 문서

- [분석 보고서](01-ANALYSIS-REPORT.md)
- [기술 조사](02-RESEARCH-REPORT.md)
- [테스트 전략](03-TEST-STRATEGY.md)
- [아키텍처 설계](04-ARCHITECTURE-DESIGN.md)
