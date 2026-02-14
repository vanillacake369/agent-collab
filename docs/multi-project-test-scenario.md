# Multi-Project Dependency Propagation Test Scenario

## 목표
Interest-based 라우팅을 통해 **파일 의존성에 따른 이벤트 전파**가 제대로 동작하는지 테스트합니다.

## 시나리오 개요

```
┌─────────────────────────────────────────────────────────────────────┐
│                    Universal Cluster                                 │
├───────────────────┬───────────────────┬───────────────────┬─────────┤
│   Agent Alice     │   Agent Bob       │   Agent Charlie   │ Agent D │
│   (auth-lib)      │   (user-service)  │   (api-gateway)   │(monitor)│
├───────────────────┼───────────────────┼───────────────────┼─────────┤
│ Interest:         │ Interest:         │ Interest:         │Interest:│
│ auth-lib/**       │ user-service/**   │ api-gateway/**    │ **      │
│                   │ auth-lib/token.go │ user-service/api/*│(all)    │
│                   │ (dependency)      │ auth-lib/jwt.go   │         │
└───────────────────┴───────────────────┴───────────────────┴─────────┘
```

## 워크스페이스 구조

에이전트들은 Docker 컨테이너 내에서 공유 볼륨(`shared-workspace`)을 사용합니다.
테스트 시 에이전트에게 프롬프트를 통해 프로젝트 구조를 생성하도록 지시합니다.

```
shared-workspace/           # Docker shared volume
├── auth-lib/              # Alice 담당 - 공유 인증 라이브러리
│   ├── go.mod
│   ├── jwt.go             # JWT 토큰 생성/검증
│   ├── token.go           # 토큰 타입 정의
│   └── middleware.go      # 인증 미들웨어
│
├── user-service/          # Bob 담당 - 사용자 서비스
│   ├── go.mod
│   ├── api/
│   │   └── handler.go     # API 핸들러 (auth-lib 사용)
│   ├── db/
│   │   └── repository.go  # DB 접근
│   └── main.go
│
├── api-gateway/           # Charlie 담당 - API 게이트웨이
│   ├── go.mod
│   ├── router.go          # 라우팅 (auth-lib, user-service 호출)
│   ├── proxy.go           # 서비스 프록시
│   └── main.go
│
└── CLAUDE.md              # 협업 가이드 (에이전트가 생성)
```

## 테스트 시나리오

### Phase 1: Interest 등록 확인
1. Alice가 `auth-lib/**` 패턴으로 Interest 등록
2. Bob이 `user-service/**`, `auth-lib/token.go` 패턴으로 Interest 등록 (의존성 추적)
3. Charlie가 `api-gateway/**`, `user-service/api/*`, `auth-lib/jwt.go` 패턴으로 Interest 등록

### Phase 2: 파일 변경 이벤트 전파 테스트

#### 시나리오 A: auth-lib/jwt.go 변경
```
Alice가 auth-lib/jwt.go 수정
  → Alice: 직접 매치 (자기 작업)
  → Bob: 매치 안됨 (token.go만 관심)
  → Charlie: 직접 매치 (auth-lib/jwt.go 의존성)
  → Agent D: 매치 (** 패턴)
```

#### 시나리오 B: auth-lib/token.go 변경
```
Alice가 auth-lib/token.go 수정
  → Alice: 직접 매치
  → Bob: 직접 매치 (의존성으로 등록)
  → Charlie: 매치 안됨
  → Agent D: 매치
```

#### 시나리오 C: user-service/api/handler.go 변경
```
Bob이 user-service/api/handler.go 수정
  → Alice: 매치 안됨
  → Bob: 직접 매치
  → Charlie: 직접 매치 (user-service/api/* 의존성)
  → Agent D: 매치
```

### Phase 3: 락 충돌 감지 테스트
```
1. Alice가 auth-lib/jwt.go 락 획득
2. Charlie가 auth-lib/jwt.go 락 시도 → 충돌 이벤트 발생
3. 충돌 이벤트가 Alice, Charlie, Agent D에게 전파되는지 확인
```

### Phase 4: 컨텍스트 공유 및 검색
```
1. Alice가 auth-lib 변경 컨텍스트 공유
2. Bob이 "authentication" 키워드로 검색 → Alice 컨텍스트 찾기
3. Charlie가 "jwt token" 키워드로 검색 → 관련 컨텍스트 찾기
```

## 검증 포인트

### 1. 이벤트 전파 정확성
- [ ] 직접 매치된 Interest에만 이벤트 전달
- [ ] 의존성으로 등록된 파일 변경도 감지
- [ ] 관심 없는 파일 변경은 필터링

### 2. 락 충돌 감지
- [ ] 동일 파일 동시 락 시도 시 충돌 감지
- [ ] 충돌 이벤트가 관련 에이전트에게만 전달

### 3. 컨텍스트 검색
- [ ] 의미적 검색으로 관련 컨텍스트 찾기
- [ ] 크로스 프로젝트 컨텍스트 공유

## 실행 방법

### 1. 환경 변수 설정
```bash
export CLAUDE_OAUTH_TOKEN=sk-ant-oat01-...
```

### 2. 클러스터 시작
```bash
make multi-up
```

### 3. 에이전트 실행

각 에이전트에게 역할별 프롬프트 주입:

```bash
# Alice: auth-lib 담당
make multi-alice

# Bob: user-service 담당 (Alice 컨텍스트 참조)
make multi-bob

# Charlie: api-gateway 담당 (Alice, Bob 컨텍스트 참조)
make multi-charlie
```

### 4. 이벤트 라우팅 확인
```bash
# 모든 에이전트의 이벤트 확인
make multi-events

# 이벤트 라우팅 검증
make multi-verify
```

### 커스텀 프롬프트 주입

`AGENT_PROMPT` 환경변수를 통해 에이전트에게 특정 작업을 지시할 수 있습니다:

```bash
# 특정 작업 지시
AGENT_PROMPT="auth-lib/jwt.go에 RefreshToken 함수를 추가하고 share_context로 공유해줘" \
  docker exec -it multi-alice docker-entrypoint run
```

### Docker Compose 구성

```yaml
services:
  alice:
    working_dir: /workspace
    environment:
      AGENT_NAME: Alice
      AGENT_COLLAB_INTERESTS: "auth-lib/**"
      AGENT_PROMPT: ${AGENT_PROMPT:-}  # 커스텀 프롬프트 지원

  bob:
    environment:
      AGENT_NAME: Bob
      AGENT_COLLAB_INTERESTS: "user-service/**,auth-lib/token.go,auth-lib/middleware.go"

  charlie:
    environment:
      AGENT_NAME: Charlie
      AGENT_COLLAB_INTERESTS: "api-gateway/**,auth-lib/jwt.go,user-service/api/*"

  monitor:
    environment:
      AGENT_NAME: Monitor
      AGENT_COLLAB_INTERESTS: "**/*"

volumes:
  shared-workspace:  # 에이전트 간 공유 볼륨
```

### Makefile 타겟

```bash
make multi-up       # 클러스터 시작
make multi-alice    # Alice 실행
make multi-bob      # Bob 실행
make multi-charlie  # Charlie 실행
make multi-events   # 이벤트 확인
make multi-verify   # 라우팅 검증
make multi-test     # 전체 테스트 실행
make multi-clean    # 정리
```

## 예상 결과

### 성공 케이스
```
[Alice] auth-lib/jwt.go 수정 → Event published
[Charlie] Received event: auth-lib/jwt.go (matched: auth-lib/jwt.go)
[Monitor] Received event: auth-lib/jwt.go (matched: **)
[Bob] (no event - not interested)
```

### 실패 케이스 (이벤트 누락)
```
[Alice] auth-lib/jwt.go 수정 → Event published
[Charlie] (no event received) ← BUG: 의존성 매칭 실패
```

## 추가 테스트 아이디어

### A. 동시 수정 시나리오
- 3명이 동시에 다른 파일 수정
- 이벤트가 올바르게 라우팅되는지 확인

### B. Interest 동적 변경
- 런타임에 Interest 추가/제거
- 변경 후 이벤트 라우팅 확인

### C. 노드 재연결
- 에이전트 재시작 후 Interest 복구
- 기존 컨텍스트 접근 가능 여부
