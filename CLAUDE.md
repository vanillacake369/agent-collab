# Agent Collaboration Guidelines

이 프로젝트는 여러 AI 에이전트가 동시에 협업하는 환경입니다.

## MCP 도구 사용 규칙

### 작업 시작 시 (필수)
1. `get_warnings` 호출 - 다른 에이전트의 활동 확인
2. `get_events` 호출 - 최근 클러스터 이벤트 확인
3. `search_similar` 호출 - 관련 컨텍스트 검색

### 파일 수정 전 (필수)
1. `acquire_lock` 호출 - 수정할 파일/영역에 락 획득
2. 락 획득 실패 시 다른 작업 진행 또는 대기

### 작업 완료 후 (필수)
1. `share_context` 호출 - 작업 내용 공유
   - file_path: 수정한 파일 경로
   - content: 변경 사항 요약 (무엇을 왜 변경했는지)
2. `release_lock` 호출 - 락 해제

### 컨텍스트 공유 형식
```
share_context({
  "file_path": "수정한 파일 경로",
  "content": "## 변경 사항\n- 변경 내용 1\n- 변경 내용 2\n\n## 이유\n변경한 이유 설명\n\n## 영향\n이 변경이 다른 부분에 미치는 영향"
})
```

### 주기적 확인 (긴 작업 시)
- 10분마다 `get_warnings` 호출하여 충돌 확인
- 관련 파일에 다른 에이전트가 락을 걸었는지 확인

## 협업 예시

### 예시 1: 새 기능 구현
```
1. get_warnings() → 경고 확인
2. search_similar("authentication") → 관련 코드 검색
3. acquire_lock("auth/handler.go", 10, 50, "JWT 토큰 검증 로직 추가")
4. ... 코드 수정 ...
5. share_context("auth/handler.go", "JWT 토큰 검증 로직 추가: 만료 시간 체크, 서명 검증 추가")
6. release_lock(lock_id)
```

### 예시 2: 버그 수정
```
1. get_events(limit=20) → 최근 변경 사항 확인
2. search_similar("error handling database") → 관련 컨텍스트 검색
3. acquire_lock("db/connection.go", 100, 150, "커넥션 풀 누수 버그 수정")
4. ... 버그 수정 ...
5. share_context("db/connection.go", "커넥션 풀 누수 수정: defer로 반드시 연결 반환하도록 변경")
6. release_lock(lock_id)
```

## 충돌 해결

락 획득 실패 시:
1. `list_locks` 호출하여 누가 락을 보유중인지 확인
2. 해당 영역 외 다른 작업 먼저 진행
3. 또는 사용자에게 충돌 상황 보고

## 중요

- **절대로** 락 없이 파일을 수정하지 마세요
- 작업 완료 후 **반드시** 컨텍스트를 공유하세요
- 다른 에이전트의 컨텍스트를 참고하여 중복 작업을 피하세요
