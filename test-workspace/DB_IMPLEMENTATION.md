# Bob의 PostgreSQL DB 구현 완료 (2026-02-13)

## 📋 구현 요약

Bob의 역할로 `main.go`의 **라인 100-159**에 PostgreSQL 연결 풀을 구현했습니다.

## ✅ 구현 완료 내역

### 1. **initDB()** 함수 (라인 100-141)
- DATABASE_URL 환경변수에서 PostgreSQL 연결 문자열 로드
- 연결 풀 설정 최적화:
  - MaxConns: 25개
  - MinConns: 5개
  - MaxConnLifetime: 1시간
  - MaxConnIdleTime: 30분
  - HealthCheckPeriod: 1분
- 데이터베이스 Ping으로 연결 검증
- 전역 변수 `dbPool`에 저장

### 2. **connectDB()** 함수 (라인 143-159)
- 초기화된 연결 풀 반환
- 5초 타임아웃으로 헬스 체크 수행
- 에러 발생 시 명확한 에러 메시지 반환

### 3. **import 추가**
- `context` 패키지 추가 (컨텍스트 기반 쿼리)
- `github.com/jackc/pgx/v5/pgxpool` 추가 (PostgreSQL 드라이버)

### 4. **전역 변수 추가**
- `dbPool *pgxpool.Pool` (라인 37): 연결 풀 저장용

## 🔗 Alice (JWT 인증)와의 연동

Bob의 DB 함수는 Alice의 `authenticate()` 함수와 완벽하게 연동됩니다:

```go
// 1. Alice의 인증 검증
isValid, err := authenticate(token)
if err != nil || !isValid {
    return fmt.Errorf("authentication failed")
}

// 2. Bob의 DB 연결 획득
pool, err := connectDB()
if err != nil {
    return fmt.Errorf("database unavailable")
}

// 3. 안전한 쿼리 실행
rows, err := pool.Query(ctx, "SELECT * FROM users WHERE id = $1", userID)
```

## 📝 Charlie (API 담당)를 위한 가이드

### 기본 사용 패턴

```go
func handleAPI(w http.ResponseWriter, r *http.Request) {
    // Step 1: 인증 (Alice)
    authHeader := r.Header.Get("Authorization")
    token := strings.TrimPrefix(authHeader, "Bearer ")
    isValid, err := authenticate(token)
    if err != nil || !isValid {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    // Step 2: DB 연결 (Bob)
    pool, err := connectDB()
    if err != nil {
        http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
        return
    }

    // Step 3: CRUD 작업
    switch r.Method {
    case http.MethodGet:
        // SELECT 쿼리
        ctx := r.Context()
        rows, err := pool.Query(ctx, "SELECT id, name FROM users")
        // ...
    case http.MethodPost:
        // INSERT 쿼리
        // ...
    }
}
```

### 파라미터화된 쿼리 (SQL Injection 방지)

```go
// ✅ 안전한 방법
pool.Query(ctx, "SELECT * FROM users WHERE email = $1", email)
pool.Exec(ctx, "INSERT INTO users (name, email) VALUES ($1, $2)", name, email)

// ❌ 위험한 방법 (절대 사용 금지)
pool.Query(ctx, fmt.Sprintf("SELECT * FROM users WHERE email = '%s'", email))
```

## 🧪 테스트 방법

### 1. PostgreSQL 실행 (Docker)
```bash
docker run --name postgres-test \
  -e POSTGRES_PASSWORD=password \
  -e POSTGRES_DB=testdb \
  -p 5432:5432 -d postgres:15
```

### 2. 테이블 생성
```sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO users (name, email) VALUES
    ('Alice', 'alice@example.com'),
    ('Bob', 'bob@example.com');
```

### 3. 환경변수 설정 및 실행
```bash
export DATABASE_URL="postgres://postgres:password@localhost:5432/testdb?sslmode=disable"
export JWT_SECRET="my-super-secret-key-at-least-32-chars-long-12345"
go run main.go
```

### 4. 예상 출력
```
JWT authentication initialized successfully
PostgreSQL connection pool initialized successfully
Pool config - MaxConns: 25, MinConns: 5
Server starting on :8080
```

## 🔐 보안 권장사항

1. **SQL Injection 방지**: 항상 파라미터화된 쿼리 사용 (`$1`, `$2`, ...)
2. **인증 우선**: DB 접근 전 반드시 `authenticate()` 호출
3. **타임아웃 설정**: 모든 쿼리에 컨텍스트 타임아웃 적용
4. **에러 처리**: 민감한 DB 에러 정보를 클라이언트에 노출하지 않기

## 📦 의존성

- `github.com/jackc/pgx/v5` v5.5.0 (go.mod에 포함)
- `context` 패키지 (Go 표준 라이브러리)

## 🎯 다음 단계 (Charlie)

Charlie가 구현해야 할 항목:
1. `setupRoutes()`: `/api/users` 엔드포인트 등록
2. `handleAPI()`: RESTful API 핸들러 구현
   - GET: 사용자 목록 조회
   - POST: 새 사용자 생성
   - PUT: 사용자 정보 수정
   - DELETE: 사용자 삭제
3. 모든 엔드포인트에 Alice의 인증 + Bob의 DB 연결 적용

## 📊 구현 상태

| 컴포넌트 | 담당자 | 상태 | 라인 범위 |
|----------|--------|------|-----------|
| JWT 인증 | Alice | ✅ 완료 | 38-98 |
| DB 연결 | Bob | ✅ 완료 | 100-159 |
| API 핸들러 | Charlie | ⏳ 대기 | 161-181 |

---
**작성자**: Bob
**구현 날짜**: 2026-02-13
**상태**: ✅ 완료
**연동 테스트**: Alice의 JWT 인증과 호환 확인 완료
