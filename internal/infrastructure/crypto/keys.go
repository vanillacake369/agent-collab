package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// KeyPair는 키 쌍입니다.
type KeyPair struct {
	PrivateKey crypto.PrivKey
	PublicKey  crypto.PubKey
	PeerID     peer.ID
}

// StoredKey는 저장된 키 형식입니다.
type StoredKey struct {
	Type       string `json:"type"`
	PrivateKey string `json:"private_key"`
	PeerID     string `json:"peer_id"`
}

// GenerateKeyPair는 새 키 쌍을 생성합니다.
func GenerateKeyPair() (*KeyPair, error) {
	privKey, pubKey, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("키 생성 실패: %w", err)
	}

	peerID, err := peer.IDFromPublicKey(pubKey)
	if err != nil {
		return nil, fmt.Errorf("Peer ID 생성 실패: %w", err)
	}

	return &KeyPair{
		PrivateKey: privKey,
		PublicKey:  pubKey,
		PeerID:     peerID,
	}, nil
}

// SaveKeyPair는 키 쌍을 파일에 저장합니다.
func SaveKeyPair(kp *KeyPair, path string) error {
	// 디렉토리 생성
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("디렉토리 생성 실패: %w", err)
	}

	// 개인키 마샬링
	privBytes, err := crypto.MarshalPrivateKey(kp.PrivateKey)
	if err != nil {
		return fmt.Errorf("개인키 마샬링 실패: %w", err)
	}

	stored := StoredKey{
		Type:       "Ed25519",
		PrivateKey: base64.StdEncoding.EncodeToString(privBytes),
		PeerID:     kp.PeerID.String(),
	}

	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON 직렬화 실패: %w", err)
	}

	// 파일 저장 (권한 600)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("파일 저장 실패: %w", err)
	}

	return nil
}

// LoadKeyPair loads a key pair from file with permission validation.
func LoadKeyPair(path string) (*KeyPair, error) {
	// Validate file permissions (should be 0600 or stricter)
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat key file: %w", err)
	}

	mode := info.Mode().Perm()
	if mode&0077 != 0 {
		return nil, fmt.Errorf("insecure key file permissions: %o (should be 0600 or stricter)", mode)
	}

	// On Unix systems, also verify file ownership
	if err := validateFileOwnership(info); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var stored StoredKey
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("JSON 파싱 실패: %w", err)
	}

	privBytes, err := base64.StdEncoding.DecodeString(stored.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("개인키 디코딩 실패: %w", err)
	}

	privKey, err := crypto.UnmarshalPrivateKey(privBytes)
	if err != nil {
		return nil, fmt.Errorf("개인키 언마샬링 실패: %w", err)
	}

	pubKey := privKey.GetPublic()

	peerID, err := peer.IDFromPublicKey(pubKey)
	if err != nil {
		return nil, fmt.Errorf("Peer ID 생성 실패: %w", err)
	}

	return &KeyPair{
		PrivateKey: privKey,
		PublicKey:  pubKey,
		PeerID:     peerID,
	}, nil
}

// KeyExists는 키 파일이 존재하는지 확인합니다.
func KeyExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// DefaultKeyPath는 기본 키 경로를 반환합니다.
func DefaultKeyPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agent-collab", "key.json"), nil
}
