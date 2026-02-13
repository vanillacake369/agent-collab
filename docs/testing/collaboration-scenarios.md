# Multi-Agent Collaboration Test Scenarios

이 문서는 Docker Compose를 통해 협업 시나리오를 테스트하는 방법을 설명합니다.

## 테스트 유형

1. **E2E 테스트** (`docker-compose.test.yml`): 자동화된 Go 테스트
2. **Claude 통합 테스트** (`docker-compose.claude.yml`): 실제 AI 에이전트 협업

---

## 환경 구성

### Prerequisites

- Docker & Docker Compose
- Go 1.21+
- make
- Anthropic API Key (Claude 통합 테스트 시)

### E2E 테스트 환경

```bash
# E2E 테스트 클러스터 시작
make e2e-up

# 클러스터 상태 확인
docker compose -f docker-compose.test.yml ps

# 로그 확인
make e2e-logs
```

### Claude 통합 테스트 환경

```bash
# 1. 클러스터 시작
make claude-up

# 2. OAuth 인증 설정 (최초 1회)
#    - URL이 출력되면 브라우저에서 열기
#    - Claude 계정으로 로그인
#    - 발급된 토큰을 터미널에 붙여넣기
make claude-auth

# 3. 에이전트 실행
make claude-alice    # 인증 담당
make claude-bob      # DB 담당
make claude-charlie  # API 담당

# 또는 순차 실행
make claude-all

# 대화형 세션
make claude-interactive-alice
```

인증 정보는 `claude-auth` 볼륨에 저장되어 모든 컨테이너가 공유합니다.

## 클러스터 구성

| 컨테이너 | Peer ID | IP Address | 역할 |
|----------|---------|------------|------|
| agent-collab-peer1 | peer1 | 172.28.0.10 | Bootstrap 노드 |
| agent-collab-peer2 | peer2 | 172.28.0.11 | Peer 노드 |
| agent-collab-peer3 | peer3 | 172.28.0.12 | Peer 노드 |

---

## 자동화 테스트 실행

### Go E2E 테스트

```bash
# 전체 E2E 테스트 (클러스터 자동 시작/종료)
make e2e-test

# 또는 클러스터를 수동으로 관리
make e2e-up
go test -v -race -tags=e2e ./src/e2e/...
make e2e-down
```

### 개별 테스트

```bash
# Lock 테스트
go test -v -race -tags=e2e ./src/e2e/ -run TestLock

# Context 동기화 테스트
go test -v -race -tags=e2e ./src/e2e/ -run TestContext

# 충돌 감지 테스트
go test -v -race -tags=e2e ./src/e2e/ -run TestConflict
```

---

## 수동 테스트 시나리오

### 시나리오 1: Lock 충돌 감지

```bash
# Terminal 1: peer1에서 lock 획득
docker exec agent-collab-peer1 /app/agent-collab mcp call acquire_lock \
  '{"file_path":"main.go","start_line":1,"end_line":50,"intention":"리팩토링"}'

# Terminal 2: peer2에서 동일 영역 lock 시도 (충돌 예상)
docker exec agent-collab-peer2 /app/agent-collab mcp call acquire_lock \
  '{"file_path":"main.go","start_line":30,"end_line":60,"intention":"수정"}'
```

### 시나리오 2: Context 동기화

```bash
# peer1에서 context 공유
docker exec agent-collab-peer1 /app/agent-collab mcp call share_context \
  '{"file_path":"auth/handler.go","content":"JWT 인증 구현 완료"}'

# peer2에서 context 검색
docker exec agent-collab-peer2 /app/agent-collab mcp call search_similar \
  '{"query":"인증 JWT authentication"}'
```

### 시나리오 3: 동시 작업 (서로 다른 파일)

```bash
# 3개 터미널에서 동시 실행
docker exec agent-collab-peer1 /app/agent-collab mcp call acquire_lock \
  '{"file_path":"api/types.go","start_line":1,"end_line":100,"intention":"타입 정의"}'

docker exec agent-collab-peer2 /app/agent-collab mcp call acquire_lock \
  '{"file_path":"domain/service.go","start_line":1,"end_line":100,"intention":"서비스 로직"}'

docker exec agent-collab-peer3 /app/agent-collab mcp call acquire_lock \
  '{"file_path":"store/repo.go","start_line":1,"end_line":100,"intention":"저장소 구현"}'
```

---

## 트러블슈팅

### 클러스터 재시작

```bash
make e2e-down
make e2e-up
```

### 전체 정리

```bash
make e2e-clean
```

### 개별 컨테이너 로그

```bash
docker logs agent-collab-peer1
docker logs agent-collab-peer2
docker logs agent-collab-peer3
```

---

## 성능 기준

| 메트릭 | 기준 |
|--------|------|
| Lock 획득 시간 | < 100ms |
| 컨텍스트 전파 시간 | < 2초 |
| 피어 연결 시간 | < 5초 |
| Lock 충돌 감지 | < 200ms |
| 이벤트 전파 | < 1초 |

---

## Claude 통합 테스트 시나리오

### 시나리오: 3 AI 에이전트 협업 개발

| 에이전트 | 컨테이너 | 역할 | 담당 함수 |
|----------|----------|------|-----------|
| Alice | claude-alice | 인증 | `authenticate`, `initAuth` |
| Bob | claude-bob | DB | `connectDB`, `initDB` |
| Charlie | claude-charlie | API | `handleAPI`, `setupRoutes` |

### 테스트 절차

1. **클러스터 시작**
   ```bash
   export ANTHROPIC_API_KEY=sk-ant-...
   make claude-up
   ```

2. **Alice 실행 (인증 구현)**
   ```bash
   make claude-alice
   # 또는 커스텀 프롬프트
   docker exec -it claude-alice claude --print "JWT 토큰 검증 함수를 구현해줘"
   ```

3. **Bob 실행 (DB 연결)**
   ```bash
   make claude-bob
   ```

4. **Charlie 실행 (API 핸들러)**
   ```bash
   make claude-charlie
   ```

5. **결과 확인**
   ```bash
   cat test-workspace/main.go
   ```

### 검증 포인트

- [ ] 각 에이전트가 `acquire_lock` 호출 후 파일 수정
- [ ] 작업 완료 후 `share_context` 호출
- [ ] 다른 에이전트의 컨텍스트 참조하여 일관된 코드 작성
- [ ] Lock 충돌 시 적절한 대응 (대기 또는 다른 영역 작업)

### 대화형 테스트

```bash
# Alice 컨테이너에서 대화형 Claude 실행
docker exec -it claude-alice claude

# Bob 컨테이너 셸 접속
make claude-shell-bob
```
