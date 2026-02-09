package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// InviteToken은 초대 토큰입니다.
type InviteToken struct {
	Version        int              `json:"v"`
	ProjectID      string           `json:"pid"`
	ProjectName    string           `json:"pn"`
	EncryptionKey  []byte           `json:"ek"`
	BootstrapPeers []BootstrapPeer  `json:"bp"`
	CreatedBy      string           `json:"cb"`
	CreatedAt      int64            `json:"ca"`
	ExpiresAt      int64            `json:"ea,omitempty"`
}

// BootstrapPeer는 bootstrap peer 정보입니다.
type BootstrapPeer struct {
	ID    string   `json:"id"`
	Addrs []string `json:"addrs"`
}

// GenerateToken은 새 초대 토큰을 생성합니다.
func GenerateToken(projectName string, createdBy string, bootstrapPeers []BootstrapPeer) (*InviteToken, error) {
	// 프로젝트 ID 생성 (32바이트 랜덤)
	projectID, err := generateRandomID(16)
	if err != nil {
		return nil, fmt.Errorf("프로젝트 ID 생성 실패: %w", err)
	}

	// 암호화 키 생성 (32바이트 = AES-256)
	encKey := make([]byte, 32)
	if _, err := rand.Read(encKey); err != nil {
		return nil, fmt.Errorf("암호화 키 생성 실패: %w", err)
	}

	return &InviteToken{
		Version:        1,
		ProjectID:      projectID,
		ProjectName:    projectName,
		EncryptionKey:  encKey,
		BootstrapPeers: bootstrapPeers,
		CreatedBy:      createdBy,
		CreatedAt:      time.Now().Unix(),
	}, nil
}

// Encode는 토큰을 문자열로 인코딩합니다.
func (t *InviteToken) Encode() (string, error) {
	data, err := json.Marshal(t)
	if err != nil {
		return "", fmt.Errorf("토큰 직렬화 실패: %w", err)
	}

	return base64.URLEncoding.EncodeToString(data), nil
}

// DecodeToken은 문자열에서 토큰을 디코딩합니다.
func DecodeToken(encoded string) (*InviteToken, error) {
	data, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("토큰 디코딩 실패: %w", err)
	}

	var token InviteToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("토큰 파싱 실패: %w", err)
	}

	return &token, nil
}

// IsExpired는 토큰이 만료되었는지 확인합니다.
func (t *InviteToken) IsExpired() bool {
	if t.ExpiresAt == 0 {
		return false // 만료 없음
	}
	return time.Now().Unix() > t.ExpiresAt
}

// SetExpiry는 만료 시간을 설정합니다.
func (t *InviteToken) SetExpiry(duration time.Duration) {
	t.ExpiresAt = time.Now().Add(duration).Unix()
}

// generateRandomID는 랜덤 ID를 생성합니다.
func generateRandomID(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length*2], nil
}

// DefaultTokenTTL is the default token expiration duration.
const DefaultTokenTTL = 24 * time.Hour

// SimpleInviteToken is a simple invite token.
type SimpleInviteToken struct {
	Addresses   []string `json:"addrs"`
	ProjectName string   `json:"project"`
	CreatorID   string   `json:"creator"`
	CreatedAt   int64    `json:"created"`
	ExpiresAt   int64    `json:"expires,omitempty"`
}

// NewInviteToken creates a new simple invite token with default expiration.
func NewInviteToken(addresses []string, projectName, creatorID string) (*SimpleInviteToken, error) {
	now := time.Now()
	return &SimpleInviteToken{
		Addresses:   addresses,
		ProjectName: projectName,
		CreatorID:   creatorID,
		CreatedAt:   now.Unix(),
		ExpiresAt:   now.Add(DefaultTokenTTL).Unix(),
	}, nil
}

// NewInviteTokenWithTTL creates a new simple invite token with custom expiration.
func NewInviteTokenWithTTL(addresses []string, projectName, creatorID string, ttl time.Duration) (*SimpleInviteToken, error) {
	now := time.Now()
	return &SimpleInviteToken{
		Addresses:   addresses,
		ProjectName: projectName,
		CreatorID:   creatorID,
		CreatedAt:   now.Unix(),
		ExpiresAt:   now.Add(ttl).Unix(),
	}, nil
}

// IsExpired checks if the token has expired.
func (t *SimpleInviteToken) IsExpired() bool {
	if t.ExpiresAt == 0 {
		return false // No expiration set
	}
	return time.Now().Unix() > t.ExpiresAt
}

// Encode encodes the token to a base64 string.
// Returns error if JSON marshaling fails.
func (t *SimpleInviteToken) Encode() (string, error) {
	data, err := json.Marshal(t)
	if err != nil {
		return "", fmt.Errorf("failed to marshal token: %w", err)
	}
	return base64.URLEncoding.EncodeToString(data), nil
}

// DecodeInviteToken은 문자열에서 토큰을 디코딩합니다.
func DecodeInviteToken(encoded string) (*SimpleInviteToken, error) {
	data, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("token decoding failed: %w", err)
	}

	var token SimpleInviteToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("token parsing failed: %w", err)
	}

	return &token, nil
}

// WireGuardInfo contains WireGuard-specific information in tokens.
type WireGuardInfo struct {
	CreatorPublicKey string `json:"cpk"`           // Creator's WireGuard public key
	CreatorEndpoint  string `json:"cep"`           // Creator's endpoint (IP:port)
	Subnet           string `json:"sub"`           // Cluster subnet
	CreatorIP        string `json:"cip"`           // Creator's VPN IP
	JoinerIP         string `json:"jip,omitempty"` // Pre-allocated IP for joiner
	PSK              string `json:"psk,omitempty"` // Optional pre-shared key
}

// WireGuardToken extends SimpleInviteToken with WireGuard support.
type WireGuardToken struct {
	// Base fields (compatible with SimpleInviteToken)
	Addresses   []string `json:"addrs"`
	ProjectName string   `json:"project"`
	CreatorID   string   `json:"creator"`
	CreatedAt   int64    `json:"created"`
	ExpiresAt   int64    `json:"expires,omitempty"`

	// WireGuard extension
	WireGuard *WireGuardInfo `json:"wg,omitempty"`
}

// NewWireGuardToken creates a new WireGuard-enabled invite token.
func NewWireGuardToken(
	addresses []string,
	projectName, creatorID string,
	wgInfo *WireGuardInfo,
) (*WireGuardToken, error) {
	now := time.Now()
	return &WireGuardToken{
		Addresses:   addresses,
		ProjectName: projectName,
		CreatorID:   creatorID,
		CreatedAt:   now.Unix(),
		ExpiresAt:   now.Add(DefaultTokenTTL).Unix(),
		WireGuard:   wgInfo,
	}, nil
}

// NewWireGuardTokenWithTTL creates a new WireGuard-enabled token with custom expiration.
func NewWireGuardTokenWithTTL(
	addresses []string,
	projectName, creatorID string,
	wgInfo *WireGuardInfo,
	ttl time.Duration,
) (*WireGuardToken, error) {
	now := time.Now()
	return &WireGuardToken{
		Addresses:   addresses,
		ProjectName: projectName,
		CreatorID:   creatorID,
		CreatedAt:   now.Unix(),
		ExpiresAt:   now.Add(ttl).Unix(),
		WireGuard:   wgInfo,
	}, nil
}

// HasWireGuard returns true if the token has WireGuard configuration.
func (t *WireGuardToken) HasWireGuard() bool {
	return t.WireGuard != nil && t.WireGuard.CreatorPublicKey != ""
}

// IsExpired checks if the token has expired.
func (t *WireGuardToken) IsExpired() bool {
	if t.ExpiresAt == 0 {
		return false
	}
	return time.Now().Unix() > t.ExpiresAt
}

// Encode encodes the token to a base64 string.
func (t *WireGuardToken) Encode() (string, error) {
	data, err := json.Marshal(t)
	if err != nil {
		return "", fmt.Errorf("failed to marshal token: %w", err)
	}
	return base64.URLEncoding.EncodeToString(data), nil
}

// ToSimpleToken converts to SimpleInviteToken (strips WireGuard info).
func (t *WireGuardToken) ToSimpleToken() *SimpleInviteToken {
	return &SimpleInviteToken{
		Addresses:   t.Addresses,
		ProjectName: t.ProjectName,
		CreatorID:   t.CreatorID,
		CreatedAt:   t.CreatedAt,
		ExpiresAt:   t.ExpiresAt,
	}
}

// DecodeWireGuardToken decodes a WireGuard-enabled token from a base64 string.
func DecodeWireGuardToken(encoded string) (*WireGuardToken, error) {
	data, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("token decoding failed: %w", err)
	}

	var token WireGuardToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("token parsing failed: %w", err)
	}

	return &token, nil
}

// DecodeAnyToken attempts to decode a token as WireGuardToken first,
// then falls back to SimpleInviteToken if no WireGuard info is present.
// Returns the token and a boolean indicating if it has WireGuard support.
func DecodeAnyToken(encoded string) (*WireGuardToken, bool, error) {
	token, err := DecodeWireGuardToken(encoded)
	if err != nil {
		// Try as simple token for backwards compatibility
		simpleToken, simpleErr := DecodeInviteToken(encoded)
		if simpleErr != nil {
			return nil, false, err // Return original error
		}
		// Convert to WireGuardToken
		return &WireGuardToken{
			Addresses:   simpleToken.Addresses,
			ProjectName: simpleToken.ProjectName,
			CreatorID:   simpleToken.CreatorID,
			CreatedAt:   simpleToken.CreatedAt,
			ExpiresAt:   simpleToken.ExpiresAt,
			WireGuard:   nil,
		}, false, nil
	}

	return token, token.HasWireGuard(), nil
}
