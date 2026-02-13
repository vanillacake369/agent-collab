# Alice의 JWT 인증 구현 완료 (2026-02-13)

## 구현 위치
- **파일**: `main.go`
- **함수**: `initAuth()` (라인 38-54), `authenticate()` (라인 58-92)
- **전역 변수**: `jwtSecret` (라인 34)

## 구현 내용

### 1. initAuth() 함수
**목적**: JWT 인증 시스템 초기화

**기능**:
- 환경변수 `JWT_SECRET`에서 비밀키 로드
- 비밀키 보안 검증 (최소 32자)
- 전역 변수 `jwtSecret`에 저장

**에러 처리**:
- JWT_SECRET 미설정 → 에러 반환
- 비밀키 길이 < 32자 → 에러 반환

**사용 예시**:
```bash
export JWT_SECRET="my-super-secret-key-at-least-32-chars-long-12345"
go run main.go
```

### 2. authenticate() 함수
**목적**: JWT 토큰 검증

**매개변수**:
- `tokenString string` - 검증할 JWT 토큰

**반환값**:
- `(bool, error)` - 인증 성공 여부와 에러 메시지

**검증 항목**:
1. JWT 토큰 파싱
2. HMAC 서명 방식 검증 (다른 방식 거부)
3. 토큰 유효성 확인
4. Claims 구조 검증
5. 만료 시간(exp claim) 확인

**보안 특징**:
- 예상치 못한 서명 알고리즘 거부
- 만료된 토큰 거부
- 필수 클레임 누락 시 거부

## Bob (DB 담당)을 위한 정보

### 연동 방법
```go
// DB 초기화 전에 인증이 필요한 경우
func initDB() error {
    // Alice의 인증은 이미 완료됨 (main에서 initAuth() 호출)

    // DB 연결 풀 초기화
    // ...
    return nil
}
```

### 미들웨어 활용
```go
// DB 접근 전 인증 체크
func secureDBMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := extractToken(r) // Authorization 헤더에서 추출
        isValid, err := authenticate(token)
        if err != nil || !isValid {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

## Charlie (API 담당)을 위한 정보

### API 핸들러에서 사용법
```go
func handleAPI(w http.ResponseWriter, r *http.Request) {
    // 1. Authorization 헤더에서 토큰 추출
    authHeader := r.Header.Get("Authorization")
    if authHeader == "" {
        http.Error(w, "Missing authorization header", http.StatusUnauthorized)
        return
    }

    // 2. "Bearer " 접두사 제거
    token := strings.TrimPrefix(authHeader, "Bearer ")

    // 3. 토큰 검증
    isValid, err := authenticate(token)
    if err != nil {
        http.Error(w, fmt.Sprintf("Authentication failed: %v", err), http.StatusUnauthorized)
        return
    }

    if !isValid {
        http.Error(w, "Invalid token", http.StatusUnauthorized)
        return
    }

    // 4. 인증 성공 - API 로직 실행
    // ... DB 쿼리, 비즈니스 로직 등
}
```

### setupRoutes() 예시
```go
func setupRoutes() {
    // 인증 미들웨어 생성
    authMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
        return func(w http.ResponseWriter, r *http.Request) {
            authHeader := r.Header.Get("Authorization")
            token := strings.TrimPrefix(authHeader, "Bearer ")

            isValid, err := authenticate(token)
            if err != nil || !isValid {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }

            next(w, r)
        }
    }

    // 보호된 API 라우트 등록
    http.HandleFunc("/api/users", authMiddleware(handleAPI))
}
```

## 의존성
- `github.com/golang-jwt/jwt/v5` v5.2.0 (go.mod에 이미 포함)

## 테스트 토큰 생성 예시
```go
// 테스트용 토큰 생성 함수 (별도 파일에 추가 가능)
func generateTestToken() (string, error) {
    claims := jwt.MapClaims{
        "sub": "user123",
        "exp": time.Now().Add(time.Hour * 24).Unix(), // 24시간 유효
        "iat": time.Now().Unix(),
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(jwtSecret)
}
```

## 다음 단계
1. **Bob**: `initDB()`, `connectDB()` 구현
   - 인증된 요청만 DB 접근 가능하도록 설계
   - PostgreSQL 연결 풀 설정

2. **Charlie**: `setupRoutes()`, `handleAPI()` 구현
   - `/api/users` 엔드포인트에 `authenticate()` 미들웨어 적용
   - GET/POST/PUT/DELETE 메서드 구현
   - 인증 실패 시 HTTP 401 반환

## 구현 완료 확인
- [x] JWT 비밀키 환경변수 로딩
- [x] 비밀키 보안 검증 (32자 최소)
- [x] JWT 토큰 파싱 및 검증
- [x] HMAC 서명 방식 검증
- [x] 만료 시간 확인
- [x] 에러 처리 및 로깅

---
**작성자**: Alice
**날짜**: 2026-02-13
**상태**: ✅ 구현 완료
