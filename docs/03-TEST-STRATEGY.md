# Phase 3: Planner Report - 테스트 전략 수립 및 평가

## 1. 테스트 시나리오 정의

### 시나리오 1: 기본 클러스터 형성

| 항목 | 내용 |
|-----|------|
| **목표** | 3개 VM이 P2P 네트워크로 연결 확인 |
| **전제조건** | VM 3개 생성, agent-collab 설치 완료 |
| **성공 기준** | 60초 내 모든 노드 연결 |

```bash
# 단계
1. VM1: agent-collab init test-project → 토큰 생성
2. VM2: agent-collab join <token>
3. VM3: agent-collab join <token>
4. ALL: agent-collab status → 3 peers 확인
```

### 시나리오 2: Lock 전파

| 항목 | 내용 |
|-----|------|
| **목표** | 한 노드의 lock이 다른 노드에 전파 확인 |
| **전제조건** | 클러스터 형성 완료 |
| **성공 기준** | 5초 내 lock 상태 동기화 |

```bash
# 단계
1. VM1: agent-collab lock acquire --file src/api.go --start 10 --end 50
2. VM2: agent-collab lock list → VM1의 lock 표시
3. VM3: agent-collab lock list → VM1의 lock 표시
4. VM1: agent-collab lock release <lock-id>
5. VM2, VM3: lock list → 빈 목록
```

### 시나리오 3: 컨텍스트 동기화

| 항목 | 내용 |
|-----|------|
| **목표** | share_context 내용이 다른 노드에서 검색 가능 |
| **전제조건** | 클러스터 형성 + Daemon 실행 |
| **성공 기준** | 임베딩 후 10초 내 검색 가능 |

```bash
# 단계
1. ALL: agent-collab daemon start
2. VM1: curl -X POST localhost/embed -d '{"text": "user authentication logic"}'
3. VM2: curl localhost/search?query="auth"
4. 결과에 VM1의 컨텍스트 포함 확인
```

### 시나리오 4: MCP 통합 (선택적)

| 항목 | 내용 |
|-----|------|
| **목표** | Claude Code MCP 도구 호출 정상 동작 |
| **전제조건** | 클러스터 + Claude Code 설치 |
| **성공 기준** | 모든 MCP 도구 정상 응답 |

## 2. 인프라 구성 계획

### VM 스펙

```yaml
instances:
  count: 3
  names: [peer1, peer2, peer3]
  image: 22.04  # Ubuntu LTS
  cpus: 2
  memory: 2G
  disk: 10G
  network: NAT (default)
```

### 필요 소프트웨어

| 소프트웨어 | 버전 | 용도 |
|-----------|------|------|
| agent-collab | latest | P2P 컨텍스트 공유 |
| Node.js | 18+ | Claude Code 의존성 |
| Claude Code | latest | MCP 클라이언트 (선택) |
| jq | any | JSON 파싱 |
| curl | any | HTTP 요청 |

### 디렉토리 구조

```
/home/ubuntu/
├── agent-collab              # 바이너리
├── project/                   # 테스트 프로젝트
│   ├── src/
│   │   └── api.go            # 테스트용 파일
│   └── .claude/
│       └── mcp.json          # MCP 설정
├── .agent-collab/            # 상태 디렉토리
└── test-results/             # 테스트 결과
```

### 네트워크 구성

```
┌─────────────────────────────────────────────────┐
│                 Host Machine                     │
│                                                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐      │
│  │  peer1   │  │  peer2   │  │  peer3   │      │
│  │ (leader) │  │ (member) │  │ (member) │      │
│  │          │  │          │  │          │      │
│  │ NAT IP   │  │ NAT IP   │  │ NAT IP   │      │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘      │
│       │             │             │             │
│       └─────────────┼─────────────┘             │
│                     │                           │
│            libp2p P2P Network                   │
│         (Bootstrap via invite token)            │
└─────────────────────────────────────────────────┘
```

## 3. 자동화 스크립트 계획

### 스크립트 목록

| 스크립트 | 기능 | 의존성 |
|---------|------|--------|
| `setup.sh` | VM 생성, 소프트웨어 설치 | multipass |
| `cluster-init.sh` | 클러스터 초기화 | setup.sh |
| `test-cluster.sh` | 클러스터 형성 테스트 | cluster-init.sh |
| `test-lock.sh` | Lock 전파 테스트 | cluster-init.sh |
| `test-context.sh` | 컨텍스트 동기화 테스트 | cluster-init.sh |
| `test-mcp.sh` | MCP 통합 테스트 (선택) | cluster-init.sh |
| `cleanup.sh` | VM 삭제, 정리 | - |
| `run-all.sh` | 전체 테스트 실행 | all |

### 실행 순서

```
┌─────────────────┐
│    setup.sh     │
└────────┬────────┘
         │
┌────────▼────────┐
│ cluster-init.sh │
└────────┬────────┘
         │
    ┌────┴────┐
    │         │
┌───▼───┐ ┌───▼───┐
│ Tier1 │ │ Tier2 │
│ tests │ │ tests │
└───┬───┘ └───┬───┘
    │         │
    └────┬────┘
         │
┌────────▼────────┐
│   cleanup.sh    │
└─────────────────┘
```

### Tier 분류

**Tier 1: Core (필수)**
- test-cluster.sh: 연결 확인
- test-lock.sh: Lock 전파

**Tier 2: Advanced (권장)**
- test-context.sh: 컨텍스트 동기화

**Tier 3: Integration (선택)**
- test-mcp.sh: Claude Code 통합

## 4. 성공 지표

### 정량적 지표

| 영역 | 지표 | 기준값 |
|-----|------|--------|
| 클러스터 | 연결 성공률 | 100% (3/3) |
| 클러스터 | 연결 시간 | < 60초 |
| Lock | 전파 성공률 | 100% |
| Lock | 전파 지연 | < 5초 |
| Context | 동기화 성공률 | > 95% |
| Context | 검색 응답 시간 | < 1초 |
| MCP | 도구 성공률 | 100% |

### Pass/Fail 기준

```
PASS:    모든 정량적 지표 충족
PARTIAL: 80% 이상 충족
FAIL:    80% 미만
```

### 결과 형식

```json
{
  "timestamp": "2025-02-09T10:30:00Z",
  "overall": "PASS",
  "tests": {
    "cluster_formation": {
      "status": "PASS",
      "duration_ms": 45000,
      "peers_connected": 3
    },
    "lock_propagation": {
      "status": "PASS",
      "propagation_time_ms": 2300
    },
    "context_sync": {
      "status": "PASS",
      "search_latency_ms": 450
    }
  }
}
```

## 5. 리스크 분석 및 대응

### Risk Matrix

| 리스크 | 확률 | 영향 | 대응책 |
|-------|------|------|--------|
| NAT Traversal 실패 | 중 | 높음 | Circuit Relay 활성화 |
| API 인증 실패 | 낮음 | 중 | 환경 변수 사전 검증 |
| 임베딩 API 지연 | 중 | 중 | Mock 임베딩 사용 |
| VM 리소스 부족 | 낮음 | 중 | RAM 4GB로 증가 |
| 환경 불일치 | 중 | 중 | 크로스 컴파일 검증 |

### Fallback 전략

```
시나리오: NAT Traversal 실패
├─ 1차: Circuit Relay 자동 활성화 (구현됨)
├─ 2차: SSH 터널 수동 구성
└─ 3차: Bridged 네트워크로 전환

시나리오: 임베딩 실패
├─ 1차: Mock 임베딩 제공자 사용
├─ 2차: Ollama 로컬 모델
└─ 3차: 임베딩 테스트 스킵
```

## 6. 전략 평가

### 평가 결과

| 기준 | 점수 | 비고 |
|-----|------|------|
| 실현 가능성 | 높음 | 모든 구성 요소 구현됨 |
| 리소스 효율 | 중 | VM 3개, 각 2GB |
| 자동화 수준 | 높음 | 스크립트 기반 |
| 신뢰성 | 중 | NAT 불확실성 |
| 확장성 | 높음 | 추가 시나리오 용이 |

### 보완 사항

1. **네트워크 모니터링**: 연결 상태 실시간 확인
2. **테스트 격리**: 시나리오 간 독립 실행
3. **결과 수집 강화**: JSON 구조화, 타임스탬프
4. **점진적 테스트**: Tier 1 → 2 → 3

## 7. 실행 계획

### Phase A: 환경 준비 (5분)
- [ ] VM 3개 생성
- [ ] 소프트웨어 설치
- [ ] 바이너리 배포

### Phase B: 클러스터 테스트 (3분)
- [ ] 클러스터 초기화
- [ ] 연결 확인

### Phase C: Lock 테스트 (2분)
- [ ] Lock 획득/전파/해제
- [ ] 결과 검증

### Phase D: Context 테스트 (3분)
- [ ] Daemon 시작
- [ ] 컨텍스트 공유/검색

### Phase E: MCP 테스트 (선택, 5분)
- [ ] Claude Code 설정
- [ ] MCP 도구 테스트

### Phase F: 정리 (2분)
- [ ] 결과 수집
- [ ] VM 삭제

**총 예상 소요: 15-20분**

## 8. 다음 단계 (Phase 4: Architect)

구현할 항목:
1. 자동화 스크립트 작성
2. 테스트 프로젝트 템플릿 생성
3. 결과 수집/보고 시스템
4. CI/CD 통합 (선택)
