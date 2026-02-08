package ctxsync

import (
	"encoding/json"
	"sync"
)

// VectorClock은 벡터 클럭입니다.
type VectorClock struct {
	mu     sync.RWMutex
	clocks map[string]uint64 // nodeID -> logical time
}

// NewVectorClock은 새 벡터 클럭을 생성합니다.
func NewVectorClock() *VectorClock {
	return &VectorClock{
		clocks: make(map[string]uint64),
	}
}

// FromMap은 맵에서 벡터 클럭을 생성합니다.
func FromMap(clocks map[string]uint64) *VectorClock {
	vc := NewVectorClock()
	for k, v := range clocks {
		vc.clocks[k] = v
	}
	return vc
}

// Increment는 특정 노드의 클럭을 증가시킵니다.
func (vc *VectorClock) Increment(nodeID string) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	vc.clocks[nodeID]++
}

// Get은 특정 노드의 클럭 값을 반환합니다.
func (vc *VectorClock) Get(nodeID string) uint64 {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	return vc.clocks[nodeID]
}

// Merge는 다른 벡터 클럭과 병합합니다.
func (vc *VectorClock) Merge(other *VectorClock) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	other.mu.RLock()
	defer other.mu.RUnlock()

	for nodeID, otherTime := range other.clocks {
		if otherTime > vc.clocks[nodeID] {
			vc.clocks[nodeID] = otherTime
		}
	}
}

// Compare는 두 벡터 클럭을 비교합니다.
// -1: vc < other (happens before)
// 0: vc || other (concurrent)
// 1: vc > other (happens after)
func (vc *VectorClock) Compare(other *VectorClock) int {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	other.mu.RLock()
	defer other.mu.RUnlock()

	less := false
	greater := false

	// 모든 노드 수집
	allNodes := make(map[string]bool)
	for k := range vc.clocks {
		allNodes[k] = true
	}
	for k := range other.clocks {
		allNodes[k] = true
	}

	for nodeID := range allNodes {
		vcTime := vc.clocks[nodeID]
		otherTime := other.clocks[nodeID]

		if vcTime < otherTime {
			less = true
		} else if vcTime > otherTime {
			greater = true
		}
	}

	if less && !greater {
		return -1
	} else if greater && !less {
		return 1
	}
	return 0 // concurrent or equal
}

// HappensBefore는 vc가 other보다 먼저 발생했는지 확인합니다.
func (vc *VectorClock) HappensBefore(other *VectorClock) bool {
	return vc.Compare(other) == -1
}

// HappensAfter는 vc가 other보다 나중에 발생했는지 확인합니다.
func (vc *VectorClock) HappensAfter(other *VectorClock) bool {
	return vc.Compare(other) == 1
}

// IsConcurrent는 두 벡터 클럭이 동시 발생인지 확인합니다.
func (vc *VectorClock) IsConcurrent(other *VectorClock) bool {
	return vc.Compare(other) == 0
}

// Clone은 벡터 클럭을 복제합니다.
func (vc *VectorClock) Clone() *VectorClock {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	clone := NewVectorClock()
	for k, v := range vc.clocks {
		clone.clocks[k] = v
	}
	return clone
}

// ToMap은 벡터 클럭을 맵으로 변환합니다.
func (vc *VectorClock) ToMap() map[string]uint64 {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	result := make(map[string]uint64)
	for k, v := range vc.clocks {
		result[k] = v
	}
	return result
}

// MarshalJSON은 JSON 직렬화를 구현합니다.
func (vc *VectorClock) MarshalJSON() ([]byte, error) {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	return json.Marshal(vc.clocks)
}

// UnmarshalJSON은 JSON 역직렬화를 구현합니다.
func (vc *VectorClock) UnmarshalJSON(data []byte) error {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	return json.Unmarshal(data, &vc.clocks)
}
