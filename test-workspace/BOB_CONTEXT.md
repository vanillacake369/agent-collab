# Bob의 PostgreSQL 연결 풀 구현 완료 (2026-02-13)

## 구현 위치
- **파일**: `main.go`
- **함수**: `initDB()` (라인 98-141), `connectDB()` (라인 143-159)
- **전역 변수**: `dbPool *pgxpool.Pool` (라인 37)

## 구현 내용

### 1. initDB() 함수
**목적**: PostgreSQL 연결 풀 초기화 및 검증

**기능**:
- 환경변수 `DATABASE_URL`에서 데이터베이스 연결 문자열 로드
- 연결 풀 설정 최적화
  - **MaxConns**: 25 (최대 연결 수)
  - **MinConns**: 5 (최소 유지 연결 수)
  - **MaxConnLifetime**: 1시간 (연결 최대 수명)
  - **MaxConnIdleTime**: 30분 (유휴 연결 타임아웃)
  - **HealthCheckPeriod**: 1분 (헬스 체크 주기)
- 연결 풀 생성 및 데이터베이스 Ping 테스트
- 전역 변수 `dbPool`에 저장

**에러 처리**:
- DATABASE_URL 미설정 → 에러 반환
- URL 파싱 실패 → 에러 반환
- 연결 풀 생성 실패 → 에러 반환
- Ping 실패 → 연결 풀 닫고 에러 반환

**사용 예시**:
```bash
export DATABASE_URL="postgres://username:password@localhost:5432/dbname?sslmode=disable"
export JWT_SECRET="my-super-secret-key-at-least-32-chars-long-12345"
go run main.go
```

### 2. connectDB() 함수
**목적**: 활성 연결 풀 반환 및 헬스 체크

**매개변수**: 없음

**반환값**:
- `(*pgxpool.Pool, error)` - 연결 풀 포인터와 에러

**기능**:
1. 연결 풀 초기화 여부 확인
2. 5초 타임아웃으로 데이터베이스 Ping 수행
3. 헬스 체크 통과 시 연결 풀 반환

**에러 처리**:
- 연결 풀 미초기화 → 에러 반환 (initDB() 먼저 호출 필요)
- Ping 실패 → 에러 반환

## Alice (JWT 인증)와의 연동

### Alice의 JWT 인증 활용
Bob의 DB 함수는 Alice가 구현한 `authenticate()` 함수와 함께 사용될 수 있습니다:

```go
func secureDBQuery(w http.ResponseWriter, r *http.Request) {
    // 1. Alice의 인증 검증
    authHeader := r.Header.Get("Authorization")
    token := strings.TrimPrefix(authHeader, "Bearer ")

    isValid, err := authenticate(token)
    if err != nil || !isValid {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    // 2. Bob의 DB 연결 획득
    pool, err := connectDB()
    if err != nil {
        http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
        return
    }

    // 3. 데이터베이스 쿼리 실행
    ctx := context.Background()
    rows, err := pool.Query(ctx, "SELECT id, name FROM users")
    if err != nil {
        http.Error(w, "Query failed", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    // 4. 결과 처리
    // ...
}
```

## Charlie (API 담당)을 위한 정보

### API 핸들러에서 DB 연결 사용법

```go
func handleAPI(w http.ResponseWriter, r *http.Request) {
    // 1. 인증 검증 (Alice의 authenticate 함수 사용)
    authHeader := r.Header.Get("Authorization")
    token := strings.TrimPrefix(authHeader, "Bearer ")

    isValid, err := authenticate(token)
    if err != nil || !isValid {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    // 2. DB 연결 획득 (Bob의 connectDB 함수 사용)
    pool, err := connectDB()
    if err != nil {
        http.Error(w, "Database error", http.StatusServiceUnavailable)
        log.Printf("DB connection error: %v", err)
        return
    }

    // 3. HTTP 메서드에 따른 처리
    switch r.Method {
    case http.MethodGet:
        // SELECT 쿼리 실행
        ctx := r.Context()
        rows, err := pool.Query(ctx, "SELECT id, name, email FROM users")
        if err != nil {
            http.Error(w, "Query failed", http.StatusInternalServerError)
            return
        }
        defer rows.Close()

        // 결과를 JSON으로 반환
        // ...

    case http.MethodPost:
        // INSERT 쿼리 실행
        ctx := r.Context()
        _, err := pool.Exec(ctx,
            "INSERT INTO users (name, email) VALUES ($1, $2)",
            "John Doe", "john@example.com")
        if err != nil {
            http.Error(w, "Insert failed", http.StatusInternalServerError)
            return
        }

        w.WriteHeader(http.StatusCreated)
        // ...

    case http.MethodPut:
        // UPDATE 쿼리 실행
        // ...

    case http.MethodDelete:
        // DELETE 쿼리 실행
        // ...

    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}
```

### 트랜잭션 사용 예시

```go
// 여러 쿼리를 트랜잭션으로 묶기
pool, err := connectDB()
if err != nil {
    return err
}

ctx := context.Background()
tx, err := pool.Begin(ctx)
if err != nil {
    return err
}
defer tx.Rollback(ctx) // 에러 발생 시 자동 롤백

// 쿼리 실행
_, err = tx.Exec(ctx, "INSERT INTO users (name) VALUES ($1)", "Alice")
if err != nil {
    return err
}

_, err = tx.Exec(ctx, "INSERT INTO logs (action) VALUES ($1)", "user_created")
if err != nil {
    return err
}

// 커밋
return tx.Commit(ctx)
```

### setupRoutes() 예시

```go
func setupRoutes() {
    // 인증 + DB 접근 미들웨어
    secureHandler := func(handler http.HandlerFunc) http.HandlerFunc {
        return func(w http.ResponseWriter, r *http.Request) {
            // Alice의 인증
            authHeader := r.Header.Get("Authorization")
            token := strings.TrimPrefix(authHeader, "Bearer ")
            isValid, err := authenticate(token)
            if err != nil || !isValid {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }

            // Bob의 DB 헬스 체크
            _, err = connectDB()
            if err != nil {
                http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
                return
            }

            // 핸들러 실행
            handler(w, r)
        }
    }

    // 보호된 API 라우트
    http.HandleFunc("/api/users", secureHandler(handleAPI))
}
```

## 의존성
- `github.com/jackc/pgx/v5` v5.5.0 (go.mod에 이미 포함)
- 컨텍스트 기반 쿼리 지원 (`context.Context`)

## 연결 풀 설정 상세

| 설정 | 값 | 설명 |
|------|------|------|
| MaxConns | 25 | 동시 최대 연결 수 (높은 부하 대비) |
| MinConns | 5 | 최소 유지 연결 수 (빠른 응답 보장) |
| MaxConnLifetime | 1시간 | 연결 재사용 최대 시간 |
| MaxConnIdleTime | 30분 | 유휴 연결 유지 시간 |
| HealthCheckPeriod | 1분 | 자동 헬스 체크 주기 |

## 보안 고려사항

1. **SQL Injection 방지**: 모든 쿼리는 파라미터화된 쿼리 사용 (`$1`, `$2`, ...)
   ```go
   // ✅ 안전한 방법
   pool.Query(ctx, "SELECT * FROM users WHERE id = $1", userID)

   // ❌ 위험한 방법 (사용 금지)
   pool.Query(ctx, fmt.Sprintf("SELECT * FROM users WHERE id = %s", userID))
   ```

2. **인증 연동**: Charlie는 반드시 Alice의 `authenticate()` 함수를 먼저 호출한 후 DB 접근
3. **컨텍스트 타임아웃**: 모든 쿼리에 컨텍스트 타임아웃 설정 권장
4. **연결 풀 재사용**: 쿼리마다 새 연결을 만들지 말고 `connectDB()`로 풀 재사용

## 다음 단계

1. **Charlie**: `setupRoutes()`, `handleAPI()` 구현
   - `/api/users` 엔드포인트 등록
   - GET: 사용자 목록 조회
   - POST: 새 사용자 생성
   - PUT: 사용자 정보 수정
   - DELETE: 사용자 삭제
   - 모든 요청에 Alice의 인증 + Bob의 DB 연결 활용

2. **테스트 데이터베이스 스키마 생성**:
   ```sql
   CREATE TABLE users (
       id SERIAL PRIMARY KEY,
       name VARCHAR(255) NOT NULL,
       email VARCHAR(255) UNIQUE NOT NULL,
       created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
   );
   ```

## 구현 완료 확인
- [x] DATABASE_URL 환경변수 로딩
- [x] 연결 풀 설정 최적화 (MaxConns, MinConns 등)
- [x] 데이터베이스 Ping 검증
- [x] 헬스 체크 함수 구현
- [x] 에러 처리 및 로깅
- [x] Alice의 인증과 연동 가능한 구조

## 테스트 방법

```bash
# 1. PostgreSQL 실행 (Docker 예시)
docker run --name postgres-test -e POSTGRES_PASSWORD=password -e POSTGRES_DB=testdb -p 5432:5432 -d postgres:15

# 2. 환경변수 설정
export DATABASE_URL="postgres://postgres:password@localhost:5432/testdb?sslmode=disable"
export JWT_SECRET="my-super-secret-key-at-least-32-chars-long-12345"

# 3. 애플리케이션 실행
go run main.go

# 예상 출력:
# JWT authentication initialized successfully
# PostgreSQL connection pool initialized successfully
# Pool config - MaxConns: 25, MinConns: 5
# Server starting on :8080
```

---
**작성자**: Bob
**날짜**: 2026-02-13
**상태**: ✅ 구현 완료
**연동**: Alice (JWT 인증) ✅, Charlie (API) 대기 중
