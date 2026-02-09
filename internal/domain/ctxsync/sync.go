package ctxsync

import (
	"context"
	"fmt"
	"sync"
	"time"

	"agent-collab/internal/domain/ast"
)

// SyncManager는 컨텍스트 동기화 관리자입니다.
type SyncManager struct {
	mu          sync.RWMutex
	nodeID      string
	nodeName    string
	vectorClock *VectorClock
	deltaLog    *DeltaLog
	peers       map[string]*PeerState
	watcher     *ast.FileWatcher

	// 콜백
	broadcastFn func(delta *Delta) error
	onConflict  func(*Conflict) error
}

// PeerState는 피어 상태입니다.
type PeerState struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	VectorClock *VectorClock `json:"vector_clock"`
	LastSeen    time.Time    `json:"last_seen"`
	IsOnline    bool         `json:"is_online"`
}

// Conflict는 동시 수정 충돌입니다.
type Conflict struct {
	FilePath    string    `json:"file_path"`
	LocalDelta  *Delta    `json:"local_delta"`
	RemoteDelta *Delta    `json:"remote_delta"`
	DetectedAt  time.Time `json:"detected_at"`
}

// NewSyncManager는 새 동기화 관리자를 생성합니다.
func NewSyncManager(nodeID, nodeName string) *SyncManager {
	return &SyncManager{
		nodeID:      nodeID,
		nodeName:    nodeName,
		vectorClock: NewVectorClock(),
		deltaLog:    NewDeltaLog(1000),
		peers:       make(map[string]*PeerState),
		watcher:     ast.NewFileWatcher(time.Second),
	}
}

// SetBroadcastFn는 브로드캐스트 함수를 설정합니다.
func (sm *SyncManager) SetBroadcastFn(fn func(delta *Delta) error) {
	sm.broadcastFn = fn
}

// SetConflictHandler는 충돌 핸들러를 설정합니다.
func (sm *SyncManager) SetConflictHandler(handler func(*Conflict) error) {
	sm.onConflict = handler
}

// Start는 동기화를 시작합니다.
func (sm *SyncManager) Start(ctx context.Context) {
	// 파일 변경 감시 콜백 등록
	sm.watcher.OnChange(func(change *ast.FileChange) error {
		return sm.handleLocalChange(change)
	})

	// 파일 감시 시작
	go sm.watcher.Start(ctx)

	// 주기적 heartbeat
	go sm.heartbeatLoop(ctx)
}

// Stop은 동기화를 중단합니다.
func (sm *SyncManager) Stop() {
	sm.watcher.Stop()
}

// WatchFile은 파일을 감시합니다.
func (sm *SyncManager) WatchFile(filePath string) error {
	return sm.watcher.Watch(filePath)
}

// WatchDir는 디렉토리를 감시합니다.
func (sm *SyncManager) WatchDir(dirPath string, extensions []string) error {
	return sm.watcher.WatchDir(dirPath, extensions)
}

// handleLocalChange는 로컬 변경을 처리합니다.
func (sm *SyncManager) handleLocalChange(change *ast.FileChange) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 벡터 클럭 증가
	sm.vectorClock.Increment(sm.nodeID)

	// 델타 생성
	var delta *Delta
	switch change.Type {
	case ast.ChangeModified:
		delta = NewFileChangeDelta(sm.nodeID, sm.nodeName, sm.vectorClock, change.FilePath, change.Diff)
	case ast.ChangeCreated:
		delta = NewFileChangeDelta(sm.nodeID, sm.nodeName, sm.vectorClock, change.FilePath, nil)
	case ast.ChangeDeleted:
		delta = NewDelta(DeltaFileChange, sm.nodeID, sm.nodeName, sm.vectorClock)
		delta.Payload.FilePath = change.FilePath
	}

	if delta != nil {
		sm.deltaLog.Append(delta)

		// 브로드캐스트
		if sm.broadcastFn != nil {
			return sm.broadcastFn(delta)
		}
	}

	return nil
}

// ReceiveDelta는 원격 델타를 수신합니다.
func (sm *SyncManager) ReceiveDelta(delta *Delta) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 이미 처리된 델타인지 확인
	if _, exists := sm.deltaLog.Get(delta.ID); exists {
		return nil
	}

	// 충돌 감지
	conflicts := sm.detectConflicts(delta)
	if len(conflicts) > 0 && sm.onConflict != nil {
		for _, conflict := range conflicts {
			if err := sm.onConflict(conflict); err != nil {
				return fmt.Errorf("conflict handler failed: %w", err)
			}
		}
	}

	// 벡터 클럭 병합
	sm.vectorClock.Merge(delta.VectorClock)
	sm.vectorClock.Increment(sm.nodeID)

	// 델타 로그에 추가
	sm.deltaLog.Append(delta)

	// 피어 상태 업데이트
	sm.updatePeerState(delta.SourceID, delta.SourceName, delta.VectorClock)

	return nil
}

// detectConflicts는 충돌을 감지합니다.
func (sm *SyncManager) detectConflicts(remoteDelta *Delta) []*Conflict {
	var conflicts []*Conflict

	if remoteDelta.Type != DeltaFileChange || remoteDelta.Payload.FilePath == "" {
		return conflicts
	}

	// 같은 파일에 대한 로컬 델타 확인
	for _, localDelta := range sm.deltaLog.GetBySource(sm.nodeID) {
		if localDelta.Type != DeltaFileChange {
			continue
		}
		if localDelta.Payload.FilePath != remoteDelta.Payload.FilePath {
			continue
		}

		// 동시 수정 확인
		if localDelta.VectorClock.IsConcurrent(remoteDelta.VectorClock) {
			conflicts = append(conflicts, &Conflict{
				FilePath:    remoteDelta.Payload.FilePath,
				LocalDelta:  localDelta,
				RemoteDelta: remoteDelta,
				DetectedAt:  time.Now(),
			})
		}
	}

	return conflicts
}

// updatePeerState는 피어 상태를 업데이트합니다.
func (sm *SyncManager) updatePeerState(peerID, peerName string, vc *VectorClock) {
	peer, exists := sm.peers[peerID]
	if !exists {
		peer = &PeerState{
			ID:   peerID,
			Name: peerName,
		}
		sm.peers[peerID] = peer
	}

	peer.VectorClock = vc.Clone()
	peer.LastSeen = time.Now()
	peer.IsOnline = true
}

// heartbeatLoop은 heartbeat 루프입니다.
func (sm *SyncManager) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sm.checkPeerHealth()
			sm.broadcastHeartbeat()
		}
	}
}

// checkPeerHealth는 피어 상태를 확인합니다.
func (sm *SyncManager) checkPeerHealth() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	timeout := 30 * time.Second
	now := time.Now()

	for _, peer := range sm.peers {
		if now.Sub(peer.LastSeen) > timeout {
			peer.IsOnline = false
		}
	}
}

// broadcastHeartbeat는 heartbeat를 브로드캐스트합니다.
func (sm *SyncManager) broadcastHeartbeat() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delta := NewAgentStatusDelta(sm.nodeID, sm.nodeName, sm.vectorClock, sm.nodeID, "online")

	if sm.broadcastFn != nil {
		sm.broadcastFn(delta)
	}
}

// GetVectorClock은 벡터 클럭을 반환합니다.
func (sm *SyncManager) GetVectorClock() *VectorClock {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.vectorClock.Clone()
}

// GetPeers는 피어 목록을 반환합니다.
func (sm *SyncManager) GetPeers() []*PeerState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	peers := make([]*PeerState, 0, len(sm.peers))
	for _, peer := range sm.peers {
		peers = append(peers, peer)
	}
	return peers
}

// GetRecentDeltas는 최근 델타를 반환합니다.
func (sm *SyncManager) GetRecentDeltas(count int) []*Delta {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.deltaLog.GetRecent(count)
}

// GetDeltasSince는 특정 시점 이후의 델타를 반환합니다.
func (sm *SyncManager) GetDeltasSince(vc *VectorClock) []*Delta {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.deltaLog.GetSince(vc)
}

// RequestSync는 특정 피어에게 동기화를 요청합니다.
func (sm *SyncManager) RequestSync(peerID string) (*SyncRequest, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	peer, exists := sm.peers[peerID]
	if !exists {
		return nil, fmt.Errorf("peer not found: %s", peerID)
	}

	return &SyncRequest{
		RequestorID:    sm.nodeID,
		RequestorName:  sm.nodeName,
		TargetID:       peerID,
		LastKnownClock: peer.VectorClock.Clone(),
		Timestamp:      time.Now(),
	}, nil
}

// HandleSyncRequest는 동기화 요청을 처리합니다.
func (sm *SyncManager) HandleSyncRequest(req *SyncRequest) *SyncResponse {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	deltas := sm.deltaLog.GetSince(req.LastKnownClock)

	return &SyncResponse{
		ResponderID:   sm.nodeID,
		ResponderName: sm.nodeName,
		Deltas:        deltas,
		CurrentClock:  sm.vectorClock.Clone(),
		Timestamp:     time.Now(),
	}
}

// SyncRequest는 동기화 요청입니다.
type SyncRequest struct {
	RequestorID    string       `json:"requestor_id"`
	RequestorName  string       `json:"requestor_name"`
	TargetID       string       `json:"target_id"`
	LastKnownClock *VectorClock `json:"last_known_clock"`
	Timestamp      time.Time    `json:"timestamp"`
}

// SyncResponse는 동기화 응답입니다.
type SyncResponse struct {
	ResponderID   string       `json:"responder_id"`
	ResponderName string       `json:"responder_name"`
	Deltas        []*Delta     `json:"deltas"`
	CurrentClock  *VectorClock `json:"current_clock"`
	Timestamp     time.Time    `json:"timestamp"`
}

// GetStats는 동기화 통계를 반환합니다.
func (sm *SyncManager) GetStats() *SyncStats {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	onlinePeers := 0
	for _, peer := range sm.peers {
		if peer.IsOnline {
			onlinePeers++
		}
	}

	return &SyncStats{
		TotalDeltas:  sm.deltaLog.Size(),
		TotalPeers:   len(sm.peers),
		OnlinePeers:  onlinePeers,
		WatchedFiles: len(sm.watcher.GetWatchedFiles()),
		VectorClock:  sm.vectorClock.ToMap(),
	}
}

// SyncStats는 동기화 통계입니다.
type SyncStats struct {
	TotalDeltas  int               `json:"total_deltas"`
	TotalPeers   int               `json:"total_peers"`
	OnlinePeers  int               `json:"online_peers"`
	WatchedFiles int               `json:"watched_files"`
	VectorClock  map[string]uint64 `json:"vector_clock"`
}
