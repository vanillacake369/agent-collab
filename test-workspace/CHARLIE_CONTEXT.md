# Charlie의 RESTful API 구현 완료 (2026-02-13)

## 📋 구현 요약

Charlie의 역할로 `main.go`의 **라인 162-348**에 RESTful API 시스템을 구현했습니다.

## ✅ 구현 완료 내역

### 1. **setupRoutes()** 함수 (라인 162-168)
- `/api/users` 엔드포인트 등록
- `/api/users/{id}` 패턴 지원 (특정 사용자 조회/수정/삭제)
- HTTP 핸들러 연결 완료

### 2. **handleAPI()** 함수 (라인 170-216)
**3단계 처리 구조**:
1. **인증 (Alice)**: Authorization 헤더에서 JWT 토큰 추출 및 검증
2. **DB 연결 (Bob)**: PostgreSQL 연결 풀 획득 및 헬스 체크
3. **라우팅**: HTTP 메서드별 핸들러 호출

**지원 HTTP 메서드**:
- `GET`: 사용자 조회
- `POST`: 사용자 생성
- `PUT`: 사용자 수정
- `DELETE`: 사용자 삭제

### 3. **handleGetUsers()** 함수 (라인 218-258)
- `GET /api/users`: 모든 사용자 목록 조회
- `GET /api/users/{id}`: 특정 사용자 조회
- JSON 응답 형식 지원

### 4. **handleCreateUser()** 함수 (라인 260-287)
- `POST /api/users`: 새 사용자 생성
- 필수 필드 검증 (name, email)
- SQL Injection 방지: 파라미터화된 쿼리 사용
- HTTP 201 Created 상태 코드 반환

### 5. **handleUpdateUser()** 함수 (라인 289-323)
- `PUT /api/users/{id}`: 사용자 정보 수정
- 파라미터화된 UPDATE 쿼리
- 존재하지 않는 사용자 처리 (404)

### 6. **handleDeleteUser()** 함수 (라인 325-348)
- `DELETE /api/users/{id}`: 사용자 삭제
- 파라미터화된 DELETE 쿼리
- 존재하지 않는 사용자 처리 (404)

## 🔗 Alice 및 Bob과의 완벽한 연동

### Alice (JWT 인증) 연동
```go
// Authorization 헤더에서 JWT 토큰 추출
authHeader := r.Header.Get("Authorization")
token := authHeader[7:] // "Bearer " 제거

// Alice의 authenticate() 함수로 검증
isValid, err := authenticate(token)
if err != nil || !isValid {
    http.Error(w, "Authentication failed", http.StatusUnauthorized)
    return
}
```

### Bob (DB 연결) 연동
```go
// Bob의 connectDB() 함수로 연결 풀 획득
pool, err := connectDB()
if err != nil {
    http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
    return
}

// Bob의 파라미터화된 쿼리 패턴 사용
pool.Query(ctx, "SELECT * FROM users WHERE id = $1", userID)
```

## 📝 API 엔드포인트 명세

### 1. GET /api/users
**설명**: 모든 사용자 목록 조회

**요청 예시**:
```bash
curl -H "Authorization: Bearer <JWT_TOKEN>" \
     http://localhost:8080/api/users
```

**응답 예시**:
```json
{
  "users": [
    {"id": 1, "name": "Alice", "email": "alice@example.com"},
    {"id": 2, "name": "Bob", "email": "bob@example.com"}
  ]
}
```

### 2. GET /api/users/{id}
**설명**: 특정 사용자 조회

**요청 예시**:
```bash
curl -H "Authorization: Bearer <JWT_TOKEN>" \
     http://localhost:8080/api/users/1
```

**응답 예시**:
```json
{"id": 1, "name": "Alice", "email": "alice@example.com"}
```

### 3. POST /api/users
**설명**: 새 사용자 생성

**요청 예시**:
```bash
curl -X POST \
     -H "Authorization: Bearer <JWT_TOKEN>" \
     -d "name=Charlie&email=charlie@example.com" \
     http://localhost:8080/api/users
```

**응답 예시** (HTTP 201):
```json
{
  "id": 3,
  "name": "Charlie",
  "email": "charlie@example.com",
  "message": "User created successfully"
}
```

### 4. PUT /api/users/{id}
**설명**: 사용자 정보 수정

**요청 예시**:
```bash
curl -X PUT \
     -H "Authorization: Bearer <JWT_TOKEN>" \
     -d "name=Charlie Updated&email=charlie.new@example.com" \
     http://localhost:8080/api/users/3
```

**응답 예시**:
```json
{
  "id": "3",
  "name": "Charlie Updated",
  "email": "charlie.new@example.com",
  "message": "User updated successfully"
}
```

### 5. DELETE /api/users/{id}
**설명**: 사용자 삭제

**요청 예시**:
```bash
curl -X DELETE \
     -H "Authorization: Bearer <JWT_TOKEN>" \
     http://localhost:8080/api/users/3
```

**응답 예시**:
```json
{"message": "User 3 deleted successfully"}
```

## 🔐 보안 구현 사항

### 1. **인증 (Alice 연동)**
- 모든 API 요청에 JWT 토큰 필수
- Authorization 헤더 검증
- 인증 실패 시 HTTP 401 반환

### 2. **SQL Injection 방지 (Bob 패턴 따름)**
- 모든 쿼리에 파라미터화된 방식 사용 (`$1`, `$2`, ...)
- 사용자 입력을 직접 SQL 문자열에 삽입하지 않음

### 3. **에러 처리**
- 데이터베이스 에러 상세 정보는 최소화하여 반환
- 클라이언트에게 명확한 에러 메시지 제공

## 🧪 전체 시스템 테스트

### 1. 환경 설정
```bash
# PostgreSQL 실행
docker run --name postgres-test \
  -e POSTGRES_PASSWORD=password \
  -e POSTGRES_DB=testdb \
  -p 5432:5432 -d postgres:15

# 테이블 생성
docker exec -it postgres-test psql -U postgres -d testdb -c "
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO users (name, email) VALUES ('Alice', 'alice@example.com');
"
```

### 2. 애플리케이션 실행
```bash
export DATABASE_URL="postgres://postgres:password@localhost:5432/testdb?sslmode=disable"
export JWT_SECRET="my-super-secret-key-at-least-32-chars-long-12345"
go run main.go
```

**예상 출력**:
```
JWT authentication initialized successfully
PostgreSQL connection pool initialized successfully
Pool config - MaxConns: 25, MinConns: 5
API routes registered: /api/users
Server starting on :8080
```

### 3. JWT 토큰 생성 (테스트용)
```bash
# Python으로 JWT 토큰 생성
python3 << 'EOF'
import jwt
import datetime

secret = "my-super-secret-key-at-least-32-chars-long-12345"
payload = {
    "user_id": 1,
    "exp": datetime.datetime.utcnow() + datetime.timedelta(hours=1)
}
token = jwt.encode(payload, secret, algorithm="HS256")
print(token)
EOF
```

### 4. API 테스트
```bash
# 토큰을 변수에 저장
TOKEN="<위에서 생성한 JWT 토큰>"

# 모든 사용자 조회
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/users

# 새 사용자 생성
curl -X POST \
     -H "Authorization: Bearer $TOKEN" \
     -d "name=Charlie&email=charlie@example.com" \
     http://localhost:8080/api/users

# 사용자 수정
curl -X PUT \
     -H "Authorization: Bearer $TOKEN" \
     -d "name=Charlie Updated&email=charlie.new@example.com" \
     http://localhost:8080/api/users/2

# 사용자 삭제
curl -X DELETE \
     -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/users/2
```

## 🎯 구현 아키텍처

```
HTTP Request
    ↓
[handleAPI] ← Charlie
    ↓
    ├─ Step 1: authenticate(token) ← Alice
    │   ├─ JWT 파싱 및 검증
    │   └─ 만료 시간 확인
    ↓
    ├─ Step 2: connectDB() ← Bob
    │   ├─ 연결 풀 반환
    │   └─ 헬스 체크
    ↓
    └─ Step 3: CRUD Operations ← Charlie
        ├─ GET: handleGetUsers()
        ├─ POST: handleCreateUser()
        ├─ PUT: handleUpdateUser()
        └─ DELETE: handleDeleteUser()
    ↓
JSON Response
```

## 📊 최종 구현 상태

| 컴포넌트 | 담당자 | 상태 | 라인 범위 |
|----------|--------|------|-----------|
| JWT 인증 | Alice | ✅ 완료 | 41-98 |
| DB 연결 | Bob | ✅ 완료 | 100-160 |
| API 핸들러 | Charlie | ✅ 완료 | 162-348 |

## 🎨 코드 스타일 일관성

Alice와 Bob의 패턴을 철저히 따랐습니다:
- **에러 처리**: `fmt.Errorf()` 사용 (Alice, Bob 패턴)
- **로깅**: `log.Println()` 사용 (Alice, Bob 패턴)
- **파라미터화된 쿼리**: `$1, $2` 플레이스홀더 (Bob 패턴)
- **컨텍스트 사용**: `r.Context()` 활용 (Bob 패턴)
- **JSON 응답**: 일관된 에러 메시지 형식

## 🚀 다음 단계 (선택 사항)

향후 개선 가능한 항목:
1. JSON 파싱 라이브러리 추가 (현재 form data만 지원)
2. 페이지네이션 구현
3. 정렬 및 필터링 기능
4. Rate limiting 추가
5. OpenAPI/Swagger 문서 생성

---
**작성자**: Charlie
**구현 날짜**: 2026-02-13
**상태**: ✅ 완료
**연동 테스트**: Alice의 JWT 인증 + Bob의 DB 연결 완벽 통합 완료
