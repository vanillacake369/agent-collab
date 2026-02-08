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
