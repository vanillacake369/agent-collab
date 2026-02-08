package protocol

import (
	"encoding/json"
	"time"
)

// MessageType은 메시지 타입입니다.
type MessageType string

const (
	// 컨텍스트 관련
	MsgContextDelta   MessageType = "context.delta"
	MsgContextRequest MessageType = "context.request"

	// 락 관련
	MsgLockIntent  MessageType = "lock.intent"
	MsgLockPrepare MessageType = "lock.prepare"
	MsgLockCommit  MessageType = "lock.commit"
	MsgLockRelease MessageType = "lock.release"
	MsgLockRenew   MessageType = "lock.renew"
	MsgLockAck     MessageType = "lock.ack"
	MsgLockNack    MessageType = "lock.nack"

	// Vibe 관련
	MsgVibeUpdate MessageType = "vibe.update"

	// Human 관련
	MsgHumanEscalation MessageType = "human.escalation"
	MsgHumanResolution MessageType = "human.resolution"
)

// Message는 기본 메시지 구조입니다.
type Message struct {
	Type      MessageType     `json:"type"`
	ID        string          `json:"id"`
	From      string          `json:"from"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

// NewMessage는 새 메시지를 생성합니다.
func NewMessage(msgType MessageType, from string, payload interface{}) (*Message, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &Message{
		Type:      msgType,
		ID:        generateMessageID(),
		From:      from,
		Timestamp: time.Now(),
		Payload:   data,
	}, nil
}

// Encode는 메시지를 바이트로 인코딩합니다.
func (m *Message) Encode() ([]byte, error) {
	return json.Marshal(m)
}

// DecodeMessage는 바이트에서 메시지를 디코딩합니다.
func DecodeMessage(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// GetPayload는 페이로드를 지정된 타입으로 디코딩합니다.
func (m *Message) GetPayload(v interface{}) error {
	return json.Unmarshal(m.Payload, v)
}

// generateMessageID는 메시지 ID를 생성합니다.
func generateMessageID() string {
	return time.Now().Format("20060102150405.000000")
}

// ContextDeltaPayload는 컨텍스트 Delta 페이로드입니다.
type ContextDeltaPayload struct {
	DeltaID   string            `json:"delta_id"`
	Timestamp map[string]uint64 `json:"timestamp"` // Vector Clock
	Added     []EmbeddingEntry  `json:"added"`
	Updated   []EmbeddingEntry  `json:"updated"`
	Removed   []string          `json:"removed"`
}

// EmbeddingEntry는 임베딩 항목입니다.
type EmbeddingEntry struct {
	ID        string    `json:"id"`
	FilePath  string    `json:"file_path"`
	Symbol    string    `json:"symbol"`
	Embedding []float32 `json:"embedding"`
	UpdatedAt time.Time `json:"updated_at"`
}

// LockIntentPayload는 락 의도 페이로드입니다.
type LockIntentPayload struct {
	Target    SemanticTarget `json:"target"`
	Intention string         `json:"intention"`
}

// SemanticTarget은 시맨틱 타겟입니다.
type SemanticTarget struct {
	Type      string `json:"type"` // function, class, module, file
	FilePath  string `json:"file_path"`
	Name      string `json:"name"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	ASTHash   string `json:"ast_hash"`
}

// LockPreparePayload는 락 준비 페이로드입니다.
type LockPreparePayload struct {
	Target SemanticTarget `json:"target"`
}

// LockCommitPayload는 락 커밋 페이로드입니다.
type LockCommitPayload struct {
	LockID       string         `json:"lock_id"`
	Target       SemanticTarget `json:"target"`
	Holder       string         `json:"holder"`
	Intention    string         `json:"intention"`
	FencingToken uint64         `json:"fencing_token"`
	ExpiresAt    time.Time      `json:"expires_at"`
}

// LockReleasePayload는 락 해제 페이로드입니다.
type LockReleasePayload struct {
	LockID string `json:"lock_id"`
}

// LockAckPayload는 락 확인 페이로드입니다.
type LockAckPayload struct {
	RequestID string `json:"request_id"`
	OK        bool   `json:"ok"`
	Reason    string `json:"reason,omitempty"`
}

// VibeUpdatePayload는 Vibe 업데이트 페이로드입니다.
type VibeUpdatePayload struct {
	VibeID        string   `json:"vibe_id"`
	Description   string   `json:"description"`
	AffectedFiles []string `json:"affected_files"`
	Status        string   `json:"status"` // active, paused, completed
}

// HumanEscalationPayload는 Human escalation 페이로드입니다.
type HumanEscalationPayload struct {
	ConflictID  string         `json:"conflict_id"`
	Type        string         `json:"type"` // lock_conflict, merge_conflict
	Description string         `json:"description"`
	Parties     []string       `json:"parties"`
	Target      SemanticTarget `json:"target"`
}
