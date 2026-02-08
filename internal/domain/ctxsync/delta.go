package ctxsync

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"agent-collab/internal/domain/ast"
)

// DeltaType은 델타 유형입니다.
type DeltaType string

const (
	DeltaFileChange   DeltaType = "file_change"
	DeltaLockAcquired DeltaType = "lock_acquired"
	DeltaLockReleased DeltaType = "lock_released"
	DeltaAgentStatus  DeltaType = "agent_status"
	DeltaCustom       DeltaType = "custom"
)

// Delta는 컨텍스트 델타입니다.
type Delta struct {
	ID          string        `json:"id"`
	Type        DeltaType     `json:"type"`
	SourceID    string        `json:"source_id"`
	SourceName  string        `json:"source_name"`
	VectorClock *VectorClock  `json:"vector_clock"`
	Timestamp   time.Time     `json:"timestamp"`
	Payload     *DeltaPayload `json:"payload"`
}

// DeltaPayload는 델타 페이로드입니다.
type DeltaPayload struct {
	// 파일 변경
	FilePath   string        `json:"file_path,omitempty"`
	FileDiff   *ast.FileDiff `json:"file_diff,omitempty"`
	FileHash   string        `json:"file_hash,omitempty"`

	// 락 정보
	LockID     string `json:"lock_id,omitempty"`
	TargetDesc string `json:"target_desc,omitempty"`
	Intention  string `json:"intention,omitempty"`

	// 에이전트 상태
	AgentID    string `json:"agent_id,omitempty"`
	AgentState string `json:"agent_state,omitempty"`

	// 커스텀 데이터
	CustomType string         `json:"custom_type,omitempty"`
	CustomData map[string]any `json:"custom_data,omitempty"`
}

// NewDelta는 새 델타를 생성합니다.
func NewDelta(deltaType DeltaType, sourceID, sourceName string, vc *VectorClock) *Delta {
	id := generateDeltaID(sourceID, time.Now())

	return &Delta{
		ID:          id,
		Type:        deltaType,
		SourceID:    sourceID,
		SourceName:  sourceName,
		VectorClock: vc.Clone(),
		Timestamp:   time.Now(),
		Payload:     &DeltaPayload{},
	}
}

// NewFileChangeDelta는 파일 변경 델타를 생성합니다.
func NewFileChangeDelta(sourceID, sourceName string, vc *VectorClock, filePath string, diff *ast.FileDiff) *Delta {
	delta := NewDelta(DeltaFileChange, sourceID, sourceName, vc)
	delta.Payload.FilePath = filePath
	delta.Payload.FileDiff = diff
	if diff != nil {
		delta.Payload.FileHash = diff.NewHash
	}
	return delta
}

// NewLockAcquiredDelta는 락 획득 델타를 생성합니다.
func NewLockAcquiredDelta(sourceID, sourceName string, vc *VectorClock, lockID, targetDesc, intention string) *Delta {
	delta := NewDelta(DeltaLockAcquired, sourceID, sourceName, vc)
	delta.Payload.LockID = lockID
	delta.Payload.TargetDesc = targetDesc
	delta.Payload.Intention = intention
	return delta
}

// NewLockReleasedDelta는 락 해제 델타를 생성합니다.
func NewLockReleasedDelta(sourceID, sourceName string, vc *VectorClock, lockID string) *Delta {
	delta := NewDelta(DeltaLockReleased, sourceID, sourceName, vc)
	delta.Payload.LockID = lockID
	return delta
}

// NewAgentStatusDelta는 에이전트 상태 델타를 생성합니다.
func NewAgentStatusDelta(sourceID, sourceName string, vc *VectorClock, agentID, state string) *Delta {
	delta := NewDelta(DeltaAgentStatus, sourceID, sourceName, vc)
	delta.Payload.AgentID = agentID
	delta.Payload.AgentState = state
	return delta
}

// generateDeltaID는 델타 ID를 생성합니다.
func generateDeltaID(sourceID string, timestamp time.Time) string {
	data := sourceID + timestamp.String()
	hash := sha256.Sum256([]byte(data))
	return "delta-" + hex.EncodeToString(hash[:6])
}

// DeltaLog는 델타 로그입니다.
type DeltaLog struct {
	deltas   []*Delta
	maxSize  int
	byID     map[string]*Delta
	bySource map[string][]*Delta
}

// NewDeltaLog는 새 델타 로그를 생성합니다.
func NewDeltaLog(maxSize int) *DeltaLog {
	if maxSize <= 0 {
		maxSize = 1000
	}

	return &DeltaLog{
		deltas:   make([]*Delta, 0),
		maxSize:  maxSize,
		byID:     make(map[string]*Delta),
		bySource: make(map[string][]*Delta),
	}
}

// Append adds a delta to the log.
func (dl *DeltaLog) Append(delta *Delta) {
	// Duplicate check
	if _, exists := dl.byID[delta.ID]; exists {
		return
	}

	dl.deltas = append(dl.deltas, delta)
	dl.byID[delta.ID] = delta
	dl.bySource[delta.SourceID] = append(dl.bySource[delta.SourceID], delta)

	// Enforce size limit
	if len(dl.deltas) > dl.maxSize {
		oldest := dl.deltas[0]
		dl.deltas = dl.deltas[1:]
		delete(dl.byID, oldest.ID)

		// Remove from bySource index by finding and removing the specific delta
		if sourceDeltas, ok := dl.bySource[oldest.SourceID]; ok {
			for i, d := range sourceDeltas {
				if d.ID == oldest.ID {
					dl.bySource[oldest.SourceID] = append(sourceDeltas[:i], sourceDeltas[i+1:]...)
					break
				}
			}
			// Clean up empty source entries to prevent map memory leak
			if len(dl.bySource[oldest.SourceID]) == 0 {
				delete(dl.bySource, oldest.SourceID)
			}
		}
	}
}

// Get은 델타를 가져옵니다.
func (dl *DeltaLog) Get(deltaID string) (*Delta, bool) {
	delta, ok := dl.byID[deltaID]
	return delta, ok
}

// GetBySource는 소스별 델타를 가져옵니다.
func (dl *DeltaLog) GetBySource(sourceID string) []*Delta {
	return dl.bySource[sourceID]
}

// GetSince는 특정 벡터 클럭 이후의 델타를 가져옵니다.
func (dl *DeltaLog) GetSince(vc *VectorClock) []*Delta {
	var result []*Delta
	for _, delta := range dl.deltas {
		if delta.VectorClock.HappensAfter(vc) || delta.VectorClock.IsConcurrent(vc) {
			result = append(result, delta)
		}
	}
	return result
}

// GetRecent는 최근 델타를 가져옵니다.
func (dl *DeltaLog) GetRecent(count int) []*Delta {
	if count <= 0 || count > len(dl.deltas) {
		count = len(dl.deltas)
	}
	return dl.deltas[len(dl.deltas)-count:]
}

// Size는 델타 수를 반환합니다.
func (dl *DeltaLog) Size() int {
	return len(dl.deltas)
}

// Clear는 로그를 비웁니다.
func (dl *DeltaLog) Clear() {
	dl.deltas = make([]*Delta, 0)
	dl.byID = make(map[string]*Delta)
	dl.bySource = make(map[string][]*Delta)
}
