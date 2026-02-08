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

// Merge merges another vector clock into this one.
// Uses Clone() to avoid potential deadlock from nested locks.
func (vc *VectorClock) Merge(other *VectorClock) {
	// Clone first to avoid deadlock if two goroutines call Merge on each other
	otherClocks := other.ToMap()

	vc.mu.Lock()
	defer vc.mu.Unlock()

	for nodeID, otherTime := range otherClocks {
		if otherTime > vc.clocks[nodeID] {
			vc.clocks[nodeID] = otherTime
		}
	}
}

// Compare compares two vector clocks.
// Returns:
//   -1: vc < other (happens before)
//    0: vc || other (concurrent or equal)
//    1: vc > other (happens after)
// Uses ToMap() to avoid potential deadlock from nested locks.
func (vc *VectorClock) Compare(other *VectorClock) int {
	// Get snapshots to avoid deadlock
	vcClocks := vc.ToMap()
	otherClocks := other.ToMap()

	less := false
	greater := false

	// Collect all node IDs
	allNodes := make(map[string]bool)
	for k := range vcClocks {
		allNodes[k] = true
	}
	for k := range otherClocks {
		allNodes[k] = true
	}

	for nodeID := range allNodes {
		vcTime := vcClocks[nodeID]
		otherTime := otherClocks[nodeID]

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
