package lock

import (
	"crypto/rand"
	"encoding/hex"
	"sync/atomic"
	"time"
)

const (
	DefaultTTL         = 30 * time.Second
	MaxTTL             = 5 * time.Minute
	HeartbeatInterval  = 10 * time.Second
	MaxRenewals        = 100
)

// SemanticLock은 시맨틱 락입니다.
type SemanticLock struct {
	ID           string          `json:"id"`
	Target       *SemanticTarget `json:"target"`
	HolderID     string          `json:"holder_id"`
	HolderName   string          `json:"holder_name"`
	Intention    string          `json:"intention"`
	FencingToken uint64          `json:"fencing_token"`
	AcquiredAt   time.Time       `json:"acquired_at"`
	ExpiresAt    time.Time       `json:"expires_at"`
	RenewCount   int             `json:"renew_count"`
}

// 전역 fencing token 카운터
var fencingTokenCounter uint64

// NewSemanticLock creates a new semantic lock with the given parameters.
// Panics if target or holderID is nil/empty (programming errors).
func NewSemanticLock(target *SemanticTarget, holderID, holderName, intention string) *SemanticLock {
	if target == nil {
		panic("target cannot be nil")
	}
	if holderID == "" {
		panic("holderID cannot be empty")
	}

	now := time.Now()

	return &SemanticLock{
		ID:           generateLockID(),
		Target:       target,
		HolderID:     holderID,
		HolderName:   holderName,
		Intention:    intention,
		FencingToken: atomic.AddUint64(&fencingTokenCounter, 1),
		AcquiredAt:   now,
		ExpiresAt:    now.Add(DefaultTTL),
		RenewCount:   0,
	}
}

// IsExpired는 락이 만료되었는지 확인합니다.
func (l *SemanticLock) IsExpired() bool {
	return time.Now().After(l.ExpiresAt)
}

// TTLRemaining은 남은 TTL을 반환합니다.
func (l *SemanticLock) TTLRemaining() time.Duration {
	remaining := time.Until(l.ExpiresAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Renew는 락을 갱신합니다.
func (l *SemanticLock) Renew() error {
	if l.RenewCount >= MaxRenewals {
		return ErrMaxRenewalsExceeded
	}

	l.ExpiresAt = time.Now().Add(DefaultTTL)
	l.RenewCount++
	return nil
}

// RenewWithTTL은 지정된 TTL로 락을 갱신합니다.
func (l *SemanticLock) RenewWithTTL(ttl time.Duration) error {
	if l.RenewCount >= MaxRenewals {
		return ErrMaxRenewalsExceeded
	}

	if ttl > MaxTTL {
		ttl = MaxTTL
	}

	l.ExpiresAt = time.Now().Add(ttl)
	l.RenewCount++
	return nil
}

// generateLockID는 락 ID를 생성합니다.
func generateLockID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return "lock-" + hex.EncodeToString(bytes)[:12]
}

// LockState는 락 상태입니다.
type LockState string

const (
	LockStateActive   LockState = "active"
	LockStatePending  LockState = "pending"
	LockStateReleased LockState = "released"
	LockStateExpired  LockState = "expired"
)

// LockResult는 락 획득 결과입니다.
type LockResult struct {
	Success bool
	Lock    *SemanticLock
	Reason  string
}

// LockConflict는 락 충돌 정보입니다.
type LockConflict struct {
	RequestedLock  *SemanticLock
	ConflictingLock *SemanticLock
	OverlapType    string // "full", "partial", "contains"
}

// NewLockConflict는 새 락 충돌을 생성합니다.
func NewLockConflict(requested, conflicting *SemanticLock) *LockConflict {
	overlapType := "partial"
	if requested.Target.Contains(conflicting.Target) {
		overlapType = "contains"
	} else if conflicting.Target.Contains(requested.Target) {
		overlapType = "full"
	}

	return &LockConflict{
		RequestedLock:   requested,
		ConflictingLock: conflicting,
		OverlapType:     overlapType,
	}
}
