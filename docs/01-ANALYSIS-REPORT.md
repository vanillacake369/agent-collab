# Phase 1: Analyst Report - 현재 시스템 분석

## 1. 시스템 완성도 요약

| 영역 | 완성도 | 상태 |
|-----|--------|------|
| P2P 통신 (libp2p/gossipsub) | 75% | 안정적 |
| MCP 서버 | 85% | 안정적 |
| Daemon 서버 | 80% | 안정적 |
| Lock Service | 80% | 안정적 |
| Context Sync | 70% | 진행중 |
| WireGuard VPN | 60% | 진행중 |
| E2E 테스트 | 60% | 불완전 |
| 문서화 | 70% | 양호 |

**전체 완성도: 74%**

## 2. Multipass 테스트를 위한 핵심 요구사항

### 2.1 필수 요구사항 (Must Have)

1. **크로스 VM P2P 연결**
   - libp2p 노드가 서로 다른 VM에서 연결 가능해야 함
   - NAT traversal 또는 bridged 네트워크 필요
   - Bootstrap peer 주소 전달 메커니즘

2. **Daemon 원격 접근**
   - 현재: Unix 소켓 (로컬만)
   - 필요: TCP 바인딩 옵션 또는 SSH 터널

3. **바이너리 배포**
   - Linux AMD64 크로스 컴파일
   - VM에 배포 자동화

4. **Claude Code MCP 통합**
   - 각 VM에 Node.js + Claude Code 설치
   - MCP 서버 설정 배포

### 2.2 권장 요구사항 (Should Have)

1. **자동화된 테스트 스크립트**
   - VM 생성/삭제
   - 클러스터 초기화/참여
   - 기능 검증

2. **네트워크 모니터링**
   - P2P 연결 상태 로깅
   - 메시지 전파 추적

3. **결과 리포팅**
   - 테스트 성공/실패 요약
   - 성능 메트릭 수집

### 2.3 선택 요구사항 (Nice to Have)

1. **WireGuard 통합 테스트**
2. **부하 테스트 (다수 VM)**
3. **장애 복구 테스트**

## 3. 현재 한계점 및 개선 필요 사항

### 3.1 P2P 통신

**현재 상태:**
- NAT traversal 전체 스택 구현됨
- mDNS 로컬 발견 + DHT 크로스 네트워크 발견

**한계:**
- mDNS는 같은 L2 네트워크에서만 작동
- Multipass 기본 NAT 모드에서 VM 간 직접 통신 불가

**개선안:**
```
Option A: Bridged 네트워크 사용
  - VM들이 같은 서브넷에 위치
  - mDNS 자동 발견 가능

Option B: Bootstrap peer 명시적 지정
  - 첫 번째 VM의 /ip4/x.x.x.x/tcp/port/p2p/peerID
  - 토큰에 Bootstrap 주소 포함 (이미 구현됨)
```

### 3.2 Daemon 서버

**현재 상태:**
- Unix 소켓 기반 IPC (`~/.agent-collab/daemon.sock`)
- 로컬 프로세스만 접근 가능

**한계:**
- 원격 모니터링 불가
- VM 외부에서 상태 조회 불가

**개선안:**
```
Option A: SSH 터널 (즉시 적용 가능)
  ssh -L 8080:/tmp/daemon.sock user@vm

Option B: TCP 바인딩 옵션 추가 (코드 변경 필요)
  daemon start --bind 0.0.0.0:8080

Option C: HTTP 프록시 (nginx/caddy)
```

### 3.3 테스트 인프라

**현재 상태:**
- E2E 테스트가 로컬 프로세스 기반
- WireGuard 테스트는 root 권한 필요

**한계:**
- 실제 네트워크 격리 테스트 불가
- VM 레벨 통합 테스트 없음

**개선안:**
```
1. Multipass 테스트 스크립트 작성
2. Docker Compose 대안 (더 빠른 반복)
3. CI/CD 파이프라인 통합
```

## 4. Multipass 네트워킹 분석

### 4.1 네트워크 모드

| 모드 | VM간 통신 | 호스트 통신 | mDNS |
|-----|----------|------------|------|
| NAT (기본) | ❌ 직접 불가 | ✅ | ❌ |
| Bridged | ✅ | ✅ | ✅ |

### 4.2 권장 구성

**Bridged 네트워크 (macOS):**
```bash
# 브릿지 네트워크로 VM 생성
multipass launch -n peer1 --network en0
multipass launch -n peer2 --network en0
multipass launch -n peer3 --network en0

# VM들이 같은 192.168.x.0/24 서브넷에 위치
```

**NAT + Bootstrap (대안):**
```bash
# 기본 NAT 모드로 생성
multipass launch -n peer1
multipass launch -n peer2
multipass launch -n peer3

# peer1에서 init, 토큰의 Bootstrap 주소를 peer2/3가 사용
# → libp2p DHT를 통해 서로 발견
```

## 5. 테스트 시나리오 정의

### 5.1 기본 연결 테스트
```
1. VM 3개 생성
2. peer1에서 init → 토큰 생성
3. peer2, peer3에서 join
4. 각 VM에서 status 확인 → 3 peers
```

### 5.2 Lock 전파 테스트
```
1. peer1에서 lock 획득
2. peer2, peer3에서 lock list → peer1의 lock 표시
3. peer1에서 lock 해제
4. peer2, peer3에서 확인
```

### 5.3 컨텍스트 동기화 테스트
```
1. peer1에서 share_context 호출
2. peer2에서 search_similar → peer1의 컨텍스트 검색
3. 동시 수정 시 충돌 감지 확인
```

### 5.4 MCP 통합 테스트
```
1. 각 VM에 Claude Code + MCP 설정
2. VM1의 Claude가 acquire_lock
3. VM2의 Claude가 get_warnings → lock 알림 수신
4. VM3에서 list_agents → 3개 에이전트 표시
```

## 6. 다음 단계 (Phase 2: Research)

조사가 필요한 항목:
1. Multipass bridged 네트워크 설정 방법 (macOS)
2. libp2p NAT traversal 실제 동작 검증
3. Claude Code MCP 서버 등록 베스트 프랙티스
4. 유사 프로젝트 벤치마킹 (Continue.dev, Aider 등)
