# Phase 4: Architect Report - 테스트 인프라 설계 및 구현

## 1. 구현 완료 항목

### 1.1 스크립트 구조

```
scripts/
├── setup.sh           # VM 생성, 소프트웨어 설치, 바이너리 배포
├── cluster-init.sh    # 클러스터 초기화 및 피어 조인
├── test-lock.sh       # Lock 전파 테스트 (Tier 1)
├── test-context.sh    # 컨텍스트 동기화 테스트 (Tier 2)
├── cleanup.sh         # VM 삭제, 임시 파일 정리
└── run-all.sh         # 전체 테스트 오케스트레이션
```

### 1.2 테스트 결과 저장

```
results/
├── cluster-init-YYYYMMDD-HHMMSS.json    # 클러스터 초기화 결과
├── test-lock-YYYYMMDD-HHMMSS.json       # Lock 테스트 결과
├── test-context-YYYYMMDD-HHMMSS.json    # Context 테스트 결과
└── summary-YYYYMMDD-HHMMSS.json         # 전체 요약
```

## 2. 실행 방법

### 2.1 사전 준비

```bash
# 스크립트 실행 권한 부여
chmod +x scripts/*.sh

# Multipass 설치 확인
multipass version

# agent-collab 빌드 (Linux AMD64)
cd ../agent-collab
GOOS=linux GOARCH=amd64 go build -o agent-collab-linux ./cmd/agent-collab
```

### 2.2 전체 테스트 실행

```bash
# 전체 테스트 (setup → init → lock → context → cleanup)
./scripts/run-all.sh

# Tier 1만 실행 (lock 테스트까지)
./scripts/run-all.sh --tier1

# VM 유지 (디버깅용)
./scripts/run-all.sh --skip-cleanup

# 기존 VM 재사용
./scripts/run-all.sh --skip-setup
```

### 2.3 개별 스크립트 실행

```bash
# 1. VM 설정
./scripts/setup.sh

# 2. 클러스터 초기화
./scripts/cluster-init.sh

# 3. Lock 테스트
./scripts/test-lock.sh

# 4. Context 테스트
./scripts/test-context.sh

# 5. 정리
./scripts/cleanup.sh
```

## 3. 아키텍처 다이어그램

### 3.1 테스트 환경 구성

```
┌─────────────────────────────────────────────────────────────────┐
│                         Host Machine (macOS)                     │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    Multipass Hypervisor                  │   │
│  │                                                          │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │   peer1     │  │   peer2     │  │   peer3     │     │   │
│  │  │  (leader)   │  │  (member)   │  │  (member)   │     │   │
│  │  │             │  │             │  │             │     │   │
│  │  │ Ubuntu 22.04│  │ Ubuntu 22.04│  │ Ubuntu 22.04│     │   │
│  │  │ 2 CPU, 2GB  │  │ 2 CPU, 2GB  │  │ 2 CPU, 2GB  │     │   │
│  │  │             │  │             │  │             │     │   │
│  │  │ ┌─────────┐ │  │ ┌─────────┐ │  │ ┌─────────┐ │     │   │
│  │  │ │ agent-  │ │  │ │ agent-  │ │  │ │ agent-  │ │     │   │
│  │  │ │ collab  │ │  │ │ collab  │ │  │ │ collab  │ │     │   │
│  │  │ └────┬────┘ │  │ └────┬────┘ │  │ └────┬────┘ │     │   │
│  │  │      │      │  │      │      │  │      │      │     │   │
│  │  └──────┼──────┘  └──────┼──────┘  └──────┼──────┘     │   │
│  │         │                │                │             │   │
│  │         └────────────────┼────────────────┘             │   │
│  │                          │                              │   │
│  │                   libp2p Gossipsub                      │   │
│  │              (P2P Context Sharing Network)              │   │
│  │                                                          │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                     Test Scripts                          │  │
│  │  setup.sh → cluster-init.sh → test-*.sh → cleanup.sh    │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### 3.2 테스트 흐름

```
┌─────────────────┐
│   run-all.sh    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│    setup.sh     │──→ VM 3개 생성
│                 │──→ agent-collab 배포
│                 │──→ 테스트 프로젝트 설정
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ cluster-init.sh │──→ peer1: init (토큰 생성)
│                 │──→ peer2, peer3: join (토큰 사용)
│                 │──→ 연결 대기 및 확인
└────────┬────────┘
         │
    ┌────┴────┐
    ▼         ▼
┌───────┐ ┌───────┐
│Tier 1 │ │Tier 2 │
│       │ │       │
│ Lock  │ │Context│
│ Test  │ │ Test  │
└───┬───┘ └───┬───┘
    │         │
    └────┬────┘
         │
         ▼
┌─────────────────┐
│  Results JSON   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   cleanup.sh    │
└─────────────────┘
```

## 4. 테스트 시나리오 상세

### 4.1 클러스터 형성 테스트

```
Input:
  - VM 3개 (peer1, peer2, peer3)
  - agent-collab 바이너리

Steps:
  1. peer1: agent-collab init "multipass-test"
     └─ 출력: agent-collab://... 토큰

  2. peer2: agent-collab join <token>
     └─ libp2p Bootstrap → peer1 연결

  3. peer3: agent-collab join <token>
     └─ libp2p Bootstrap → peer1, peer2 연결

  4. ALL: agent-collab status
     └─ 검증: 3 peers connected

Expected:
  - 60초 내 연결 완료
  - 모든 노드에서 3 peers 표시
```

### 4.2 Lock 전파 테스트

```
Input:
  - 클러스터 형성 완료
  - 테스트 파일: src/api.go

Steps:
  1. peer1: lock acquire --file src/api.go --start 10 --end 50
     └─ Gossipsub: LockIntent 브로드캐스트

  2. peer2, peer3: lock list
     └─ 검증: peer1의 lock 표시

  3. peer1: lock release
     └─ Gossipsub: LockReleased 브로드캐스트

  4. peer2, peer3: lock list
     └─ 검증: 빈 목록

Expected:
  - 5초 내 lock 전파
  - 모든 노드에서 일관된 상태
```

### 4.3 컨텍스트 동기화 테스트

```
Input:
  - 클러스터 + Daemon 실행
  - 테스트 컨텍스트: "Authentication module..."

Steps:
  1. ALL: daemon start

  2. peer1: embed/share context
     └─ 임베딩 생성 + 벡터 저장

  3. peer2: search similar "authentication"
     └─ 검증: peer1 컨텍스트 검색됨

  4. ALL: list agents
     └─ 검증: 3개 에이전트 표시

Expected:
  - 10초 내 검색 가능
  - 임베딩 벡터 동기화
```

## 5. 결과 형식

### 5.1 개별 테스트 결과

```json
{
  "test": "lock_propagation",
  "timestamp": "2025-02-09T10:30:00Z",
  "overall": "PASS",
  "results": {
    "acquire_lock": "PASS",
    "acquire_lock_duration_ms": 1200,
    "lock_propagation": "PASS",
    "propagation_time_sec": 3,
    "release_lock": "PASS",
    "release_propagation": "PASS"
  }
}
```

### 5.2 전체 요약

```json
{
  "test_suite": "multipass_context_sharing",
  "timestamp": "2025-02-09T10:35:00Z",
  "overall": "PASS",
  "total_duration_sec": 180,
  "summary": {
    "passed": 4,
    "failed": 0,
    "partial": 0
  },
  "phases": {
    "setup": "PASS",
    "cluster_init": "PASS",
    "lock_test": "PASS",
    "context_test": "PASS"
  }
}
```

## 6. 트러블슈팅 가이드

### 6.1 VM 생성 실패

```bash
# 원인: Multipass 서비스 문제
sudo launchctl stop com.canonical.multipassd
sudo launchctl start com.canonical.multipassd

# 원인: 디스크 공간 부족
multipass list  # 기존 VM 확인
multipass delete --purge <vm-name>
```

### 6.2 피어 연결 실패

```bash
# VM 간 네트워크 확인
multipass exec peer1 -- ping -c 3 $(multipass info peer2 | grep IPv4 | awk '{print $2}')

# libp2p 로그 확인
multipass exec peer1 -- cat /tmp/daemon.log | grep -i "peer\|connect"

# 방화벽 확인
multipass exec peer1 -- sudo ufw status
```

### 6.3 Lock 전파 실패

```bash
# Gossipsub 상태 확인
multipass exec peer1 -- /home/ubuntu/agent-collab status --verbose

# 수동 lock 확인
multipass exec peer2 -- /home/ubuntu/agent-collab lock list --all
```

### 6.4 Daemon 시작 실패

```bash
# 포트 충돌 확인
multipass exec peer1 -- ss -tlnp | grep 8080

# 수동 시작 (포그라운드)
multipass exec peer1 -- /home/ubuntu/agent-collab daemon start --foreground
```

## 7. 확장 방안

### 7.1 추가 테스트 시나리오

- [ ] Lock 충돌 해결 테스트
- [ ] 네트워크 파티션 복구 테스트
- [ ] 장시간 실행 안정성 테스트
- [ ] 동시 수정 CRDT 병합 테스트

### 7.2 CI/CD 통합

```yaml
# .github/workflows/e2e-multipass.yml
name: E2E Multipass Tests
on: [push, pull_request]
jobs:
  e2e:
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install Multipass
        run: brew install multipass
      - name: Build Binary
        run: GOOS=linux GOARCH=amd64 go build -o agent-collab-linux ./cmd/agent-collab
      - name: Run E2E Tests
        run: ./scripts/run-all.sh --tier1
      - name: Upload Results
        uses: actions/upload-artifact@v4
        with:
          name: test-results
          path: results/
```

### 7.3 Docker 대안

빠른 반복을 위한 Docker Compose 기반 테스트:

```yaml
# docker-compose.test.yml
version: '3.8'
services:
  peer1:
    build: .
    command: agent-collab init test-project
    networks:
      - p2p-network
  peer2:
    build: .
    depends_on: [peer1]
    command: agent-collab join ${TOKEN}
    networks:
      - p2p-network
  peer3:
    build: .
    depends_on: [peer1]
    command: agent-collab join ${TOKEN}
    networks:
      - p2p-network
networks:
  p2p-network:
    driver: bridge
```

## 8. 다음 단계

구현 완료 후 실행 순서:

1. **스크립트 권한 설정**
   ```bash
   chmod +x scripts/*.sh
   ```

2. **바이너리 빌드**
   ```bash
   cd ../agent-collab
   GOOS=linux GOARCH=amd64 go build -o agent-collab-linux ./cmd/agent-collab
   ```

3. **테스트 실행**
   ```bash
   ./scripts/run-all.sh
   ```

4. **결과 확인**
   ```bash
   cat results/summary-*.json | jq .
   ```
