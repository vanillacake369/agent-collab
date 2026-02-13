# Agent Collaboration Test Project

이 프로젝트는 3명의 AI 에이전트(Alice, Bob, Charlie)가 agent-collab MCP를 통해 협업하는 테스트입니다.

## 필수: MCP 도구 사용 규칙

### 작업 시작 전 (반드시 실행)
```
1. get_warnings() - 다른 에이전트의 활동/경고 확인
2. search_similar("관련 키워드") - 다른 에이전트가 공유한 컨텍스트 검색
```

### 파일 수정 전 (반드시 실행)
```
3. acquire_lock(file_path, start_line, end_line, intention) - 수정할 영역 락 획득
   - 락 획득 실패 시: 다른 영역 작업 또는 대기
```

### 작업 완료 후 (반드시 실행)
```
4. share_context(file_path, content) - 작업 내용을 다른 에이전트와 공유
   - content에는 변경 사항, 이유, 사용법 포함
5. release_lock(lock_id) - 락 해제
```

## 개발자 역할 및 담당 함수

| 에이전트 | 역할 | 담당 함수 | 라인 범위 |
|----------|------|-----------|-----------|
| Alice | 인증 | `initAuth()`, `authenticate()` | 26-38 |
| Bob | DB | `initDB()`, `connectDB()` | 40-52 |
| Charlie | API | `setupRoutes()`, `handleAPI()` | 54-66 |

## 협업 시나리오

### Alice (첫 번째)
1. `get_warnings()` 호출
2. `acquire_lock("main.go", 26, 38, "JWT 인증 구현")`
3. `initAuth()`, `authenticate()` 구현
4. `share_context("main.go", "JWT 인증 구현 완료: ...")`
5. `release_lock(lock_id)`

### Bob (두 번째)
1. `get_warnings()` 호출
2. `search_similar("authentication JWT")` - Alice 작업 확인
3. `acquire_lock("main.go", 40, 52, "PostgreSQL 연결 구현")`
4. Alice의 인증과 연동되는 DB 연결 구현
5. `share_context()`, `release_lock()`

### Charlie (세 번째)
1. `search_similar("authentication database")` - Alice, Bob 작업 확인
2. `acquire_lock("main.go", 54, 66, "API 핸들러 구현")`
3. 인증/DB와 연동되는 API 핸들러 구현
4. `share_context()`, `release_lock()`

## 주의사항

- **절대** 락 없이 파일 수정 금지
- **반드시** 작업 전 `search_similar`로 기존 컨텍스트 확인
- **반드시** 작업 후 `share_context`로 내용 공유
- 다른 에이전트의 코드 스타일/패턴 따르기
