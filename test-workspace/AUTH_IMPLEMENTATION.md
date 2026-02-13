# JWT Authentication Implementation (Alice)

## 구현 완료: initAuth() 및 authenticate() 함수

### 파일: main.go (라인 33-80)
### 업데이트: 2026-02-13 (최종 구현 완료)

## 주요 구현 내용

### 1. initAuth() 함수 (라인 36-51)
- **목적**: JWT 인증 시스템 초기화
- **기능**:
  - 환경변수 `JWT_SECRET`에서 비밀키 로드
  - 비밀키 검증 (최소 32자 길이)
  - 전역 변수 `jwtSecret`에 저장
- **에러 처리**:
  - JWT_SECRET 미설정 시 에러 반환
  - 비밀키 길이 부족 시 에러 반환

### 2. authenticate() 함수 (라인 55-86)
- **목적**: JWT 토큰 검증
- **매개변수**: `tokenString string` - 검증할 JWT 토큰
- **반환값**: `(bool, error)` - 인증 성공 여부와 에러
- **기능**:
  - JWT 토큰 파싱
  - HMAC 서명 방식 검증
  - 토큰 유효성 확인
  - 만료 시간(exp claim) 검증
- **보안 검증**:
  - 예상치 못한 서명 방식 거부
  - 만료된 토큰 거부
  - 필수 클레임 누락 시 거부

## 사용법

### 환경 설정
```bash
export JWT_SECRET="your-secret-key-at-least-32-characters-long"
```

### Bob (DB 구현자)를 위한 정보
- `authenticate()` 함수를 미들웨어에서 호출하여 사용자 인증 가능
- 인증 성공 시 DB 접근 허용, 실패 시 401 Unauthorized 반환 권장

### Charlie (API 구현자)를 위한 정보
- API 핸들러에서 HTTP 헤더의 Authorization 토큰 추출 필요
  ```go
  authHeader := r.Header.Get("Authorization")
  token := strings.TrimPrefix(authHeader, "Bearer ")
  isValid, err := authenticate(token)
  ```
- 인증 실패 시 HTTP 401 응답 반환
- 인증 성공 시 DB 쿼리 및 API 로직 실행

## 의존성
- `github.com/golang-jwt/jwt/v5` (이미 go.mod에 포함됨)

## 다음 단계
1. Bob: `initDB()`, `connectDB()` 구현 시 인증된 사용자만 DB 접근 가능하도록 설계
2. Charlie: `setupRoutes()`, `handleAPI()` 구현 시 `authenticate()` 함수를 미들웨어로 활용

## 작성자
Alice (2026-02-13)
