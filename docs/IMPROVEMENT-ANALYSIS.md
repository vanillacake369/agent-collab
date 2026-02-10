# Agent-Collab 개선사항 분석

## 현재 구현 상태 요약

### P2P 네트워크 스택
- **프로토콜**: libp2p (TCP + QUIC-v1)
- **메시징**: GossipSub (Flood Publish + Peer Exchange)
- **발견**: mDNS + Kademlia DHT
- **보안**: Noise Protocol (암호화), Ed25519 (인증)
- **NAT**: UPnP, AutoNAT v2, Hole Punching, Relay Service

### 토픽 구조
```
/agent-collab/{project}/context  - 컨텍스트 동기화
/agent-collab/{project}/lock     - 락 조정
/agent-collab/{project}/vibe     - 상태 업데이트
/agent-collab/{project}/human    - 에스컬레이션
```

---

## 1. 네트워크 토폴로지 개선

### 현재 상태
- Full Mesh 지향 (모든 피어가 서로 연결)
- mDNS로 로컬 네트워크 피어 발견
- DHT로 원격 피어 발견
- 연결 수: 100-400 (LowWater-HighWater)

### 문제점

| 문제 | 영향 | 심각도 |
|------|------|--------|
| Full Mesh 확장성 한계 | N개 피어 → N*(N-1)/2 연결 | 높음 |
| 단일 부트스트랩 의존 | 부트스트랩 장애 시 조인 불가 | 중간 |
| 지역성 미고려 | 원거리 피어도 동일 취급 | 중간 |
| 피어 품질 미분류 | 느린 피어가 전파 지연 | 낮음 |

### 개선안

#### 1.1 계층적 토폴로지 (Hierarchical Topology)

```
                    [Super Peer A]
                   /      |       \
            [Peer 1]  [Peer 2]  [Peer 3]
                          |
                    [Super Peer B]
                   /      |       \
            [Peer 4]  [Peer 5]  [Peer 6]
```

**구현 방안**:
```go
type PeerRole int
const (
    RoleLeaf PeerRole = iota  // 일반 피어: 1-3개 슈퍼피어에만 연결
    RoleSuper                  // 슈퍼피어: 다수 리프 + 다른 슈퍼피어 연결
)

// 슈퍼피어 선출 기준
type SuperPeerCriteria struct {
    UptimeMin      time.Duration  // 최소 업타임 (e.g., 30분)
    BandwidthMbps  float64        // 최소 대역폭 (e.g., 10 Mbps)
    Latency99p     time.Duration  // 99p 지연시간 (e.g., < 100ms)
    ConnectionsMin int            // 최소 연결 수 (e.g., 10)
}
```

**효과**:
- 연결 복잡도: O(N²) → O(N)
- 메시지 홉: 평균 2-3 홉으로 제한
- 확장성: 1000+ 피어 지원 가능

#### 1.2 지역성 기반 클러스터링 (Locality-Aware Clustering)

```go
type PeerLocality struct {
    Region     string        // e.g., "ap-northeast-2"
    Datacenter string        // e.g., "aws-seoul-az1"
    RTT        time.Duration // 측정된 RTT
}

// 지역 내 피어 우선 연결
func (n *Node) selectPeersForMesh() []peer.ID {
    local := n.peers.FilterByRegion(n.myRegion)
    remote := n.peers.FilterByRegion("*").Exclude(local)

    // 로컬 80%, 원격 20% 비율로 메시 구성
    return append(
        local.TopK(int(0.8 * meshSize)),
        remote.TopK(int(0.2 * meshSize))...,
    )
}
```

**효과**:
- 지역 내 전파: < 10ms
- 지역 간 전파: 1개 게이트웨이 피어 경유
- 대역폭 비용 절감: 원격 트래픽 80% 감소

#### 1.3 다중 부트스트랩 전략

```go
type BootstrapConfig struct {
    // 우선순위별 부트스트랩 소스
    Sources []BootstrapSource `json:"sources"`
}

type BootstrapSource struct {
    Type     string   // "token", "dns", "dht", "hardcoded"
    Priority int      // 1-100
    Addrs    []string // 멀티어드레스
}

// DNS 기반 부트스트랩 (장애 복원력)
func (n *Node) bootstrapFromDNS(domain string) ([]peer.AddrInfo, error) {
    // _dnsaddr.bootstrap.agent-collab.io → TXT 레코드에서 피어 정보
    records, _ := net.LookupTXT("_dnsaddr." + domain)
    return parseMultiaddrs(records)
}
```

---

## 2. 메시지 브로드캐스트 개선

### 현재 상태
- GossipSub FloodPublish 모드
- 개별 JSON 메시지 전송
- 압축/배칭 없음
- 메시지 크기 제한 없음

### 문제점

| 문제 | 영향 | 심각도 |
|------|------|--------|
| 메시지 오버헤드 | 작은 변경도 전체 JSON 전송 | 높음 |
| 배칭 없음 | 고빈도 변경 시 네트워크 포화 | 높음 |
| 중복 전송 | 동일 컨텐츠 여러 번 전파 | 중간 |
| 대용량 컨텍스트 | 큰 파일 변경 시 전파 지연 | 중간 |

### 개선안

#### 2.1 메시지 배칭 (Message Batching)

```go
type MessageBatcher struct {
    buffer    []Message
    maxSize   int           // 최대 배치 크기 (e.g., 100)
    maxDelay  time.Duration // 최대 대기 시간 (e.g., 50ms)
    flushChan chan struct{}
}

func (b *MessageBatcher) Add(msg Message) {
    b.mu.Lock()
    b.buffer = append(b.buffer, msg)

    if len(b.buffer) >= b.maxSize {
        b.flushLocked()
    }
    b.mu.Unlock()
}

func (b *MessageBatcher) Start(ctx context.Context) {
    ticker := time.NewTicker(b.maxDelay)
    for {
        select {
        case <-ticker.C:
            b.Flush()
        case <-b.flushChan:
            b.Flush()
        case <-ctx.Done():
            return
        }
    }
}
```

**효과**:
- 메시지 수: 100 msg/s → 10 batch/s (90% 감소)
- 네트워크 오버헤드: 헤더 공유로 50% 감소
- 처리량: 3-5x 향상

#### 2.2 델타 압축 (Delta Compression)

```go
type CompressedMessage struct {
    Algorithm string // "zstd", "lz4", "none"
    Original  int    // 원본 크기
    Data      []byte // 압축된 데이터
}

func (n *Node) Publish(topic string, msg []byte) error {
    // 1KB 이상만 압축
    if len(msg) > 1024 {
        compressed := zstd.Compress(nil, msg)
        if len(compressed) < len(msg)*0.8 { // 20% 이상 압축 시에만
            msg = wrapCompressed("zstd", compressed, len(msg))
        }
    }
    return n.pubsub.Publish(topic, msg)
}
```

**효과**:
- 컨텍스트 메시지: 60-80% 크기 감소
- 대역폭 절약: 평균 50%
- 지연시간: 대용량 메시지 전파 2-3x 빠름

#### 2.3 우선순위 기반 전파 (Priority-Based Propagation)

```go
type MessagePriority int
const (
    PriorityLow      MessagePriority = 0  // Context updates
    PriorityNormal   MessagePriority = 1  // Lock releases
    PriorityHigh     MessagePriority = 2  // Lock intents
    PriorityCritical MessagePriority = 3  // Lock conflicts
)

type PrioritizedMessage struct {
    Priority  MessagePriority
    Deadline  time.Time // 전파 데드라인
    Payload   []byte
}

// 고우선순위 메시지 먼저 전송
func (q *MessageQueue) Dequeue() *PrioritizedMessage {
    // Critical → High → Normal → Low 순서
    for p := PriorityCritical; p >= PriorityLow; p-- {
        if msg := q.queues[p].Pop(); msg != nil {
            return msg
        }
    }
    return nil
}
```

**효과**:
- 락 충돌 알림: 즉시 전파 (< 10ms)
- 컨텍스트 업데이트: 배칭 후 전파
- 사용자 경험: 충돌 감지 시간 70% 단축

#### 2.4 컨텐츠 주소 지정 (Content-Addressed Deduplication)

```go
type ContentStore struct {
    store map[string][]byte // CID → content
}

func (cs *ContentStore) Put(content []byte) string {
    cid := multihash.Sum(content, multihash.SHA2_256, -1)
    cs.store[cid.String()] = content
    return cid.String()
}

// 대용량 컨텍스트는 CID만 전파
type ContextReference struct {
    CID      string `json:"cid"`
    Size     int    `json:"size"`
    FilePath string `json:"file_path"`
}

// 필요한 피어만 컨텐츠 요청
func (n *Node) RequestContent(cid string) ([]byte, error) {
    // DHT에서 제공자 찾기
    providers := n.dht.FindProviders(cid)
    // 가장 가까운 피어에서 다운로드
    return n.fetchFromPeer(providers[0], cid)
}
```

**효과**:
- 중복 전송 제거: 동일 컨텐츠 1회만 전파
- 대용량 파일: 필요한 피어만 다운로드
- 저장 효율: 공유 컨텐츠 1개만 저장

---

## 3. GossipSub 파라미터 튜닝

### 현재 상태
- 기본 GossipSub 설정 사용
- D=8, Dlo=6, Dhi=12 (기본값)
- FloodPublish 활성화
- PeerExchange 활성화

### 개선안

#### 3.1 클러스터 규모별 프로필

```go
type GossipProfile string
const (
    ProfileSmall  GossipProfile = "small"  // 2-10 피어
    ProfileMedium GossipProfile = "medium" // 10-50 피어
    ProfileLarge  GossipProfile = "large"  // 50-200 피어
    ProfileXLarge GossipProfile = "xlarge" // 200+ 피어
)

var ProfileParams = map[GossipProfile]pubsub.GossipSubParams{
    ProfileSmall: {
        D:            4,    // 메시 정도
        Dlo:          2,    // 최소
        Dhi:          6,    // 최대
        Dlazy:        4,    // 지연 전파
        HeartbeatInterval: 700 * time.Millisecond,
        HistoryGossip:     3,
        HistoryLength:     5,
    },
    ProfileMedium: {
        D:            6,
        Dlo:          4,
        Dhi:          8,
        Dlazy:        6,
        HeartbeatInterval: 1 * time.Second,
        HistoryGossip:     4,
        HistoryLength:     6,
    },
    ProfileLarge: {
        D:            8,
        Dlo:          6,
        Dhi:          12,
        Dlazy:        8,
        HeartbeatInterval: 1 * time.Second,
        HistoryGossip:     5,
        HistoryLength:     8,
        FanoutTTL:         60 * time.Second,
    },
}
```

#### 3.2 적응형 메시 관리 (Adaptive Mesh)

```go
type AdaptiveMesh struct {
    metrics   *MeshMetrics
    threshold AdaptiveThreshold
}

type MeshMetrics struct {
    MessageLatency  *prometheus.HistogramVec
    DeliveryRate    *prometheus.GaugeVec
    PeerChurnRate   *prometheus.GaugeVec
    DuplicateRatio  *prometheus.GaugeVec
}

func (am *AdaptiveMesh) Adjust() {
    // 지연시간 증가 → D 증가
    if am.metrics.p99Latency > 100*time.Millisecond {
        am.increaseD()
    }

    // 중복률 높음 → D 감소
    if am.metrics.duplicateRatio > 0.3 {
        am.decreaseD()
    }

    // 전달률 낮음 → Dlazy 증가
    if am.metrics.deliveryRate < 0.99 {
        am.increaseDlazy()
    }
}
```

---

## 4. 지연시간(Latency) 최적화

### 현재 상태
- 평균 전파 지연: 50-200ms (네트워크 조건에 따라)
- 락 협상 타임아웃: Intent 5초, 총 30초
- 메시지 처리: 동기 JSON 파싱

### 문제점

| 경로 | 현재 지연 | 목표 지연 |
|------|----------|----------|
| 로컬 락 획득 | 10-50ms | < 5ms |
| 원격 락 동기화 | 100-500ms | < 50ms |
| 컨텍스트 전파 | 200-1000ms | < 100ms |
| 검색 결과 | 50-200ms | < 30ms |

### 개선안

#### 4.1 로컬 우선 처리 (Local-First)

```go
type LocalFirstLock struct {
    localStore   *LockStore       // 로컬 락 저장소
    remoteSync   chan LockEvent   // 비동기 원격 동기화
}

func (l *LocalFirstLock) Acquire(target SemanticTarget) (*Lock, error) {
    // 1. 로컬 즉시 락 (< 1ms)
    lock, err := l.localStore.TryAcquire(target)
    if err != nil {
        return nil, err
    }

    // 2. 비동기 원격 브로드캐스트
    go func() {
        l.remoteSync <- LockEvent{Type: "acquired", Lock: lock}
    }()

    // 3. 충돌 발생 시 롤백
    go l.handleConflicts(lock)

    return lock, nil  // 즉시 반환
}
```

**효과**:
- 로컬 락 획득: 50ms → 1ms
- 사용자 체감 지연: 90% 감소

#### 4.2 예측적 프리페치 (Predictive Prefetch)

```go
type ContextPrefetcher struct {
    accessHistory *LRUCache  // 최근 접근 파일
    prefetchQueue chan string
}

func (cp *ContextPrefetcher) OnFileAccess(path string) {
    // 관련 파일 예측
    related := cp.predictRelated(path)

    for _, r := range related {
        if !cp.cache.Has(r) {
            cp.prefetchQueue <- r
        }
    }
}

func (cp *ContextPrefetcher) predictRelated(path string) []string {
    // 1. 같은 패키지 파일
    // 2. import하는 파일
    // 3. 자주 함께 수정된 파일 (코-수정 패턴)
    return cp.coModificationGraph.GetRelated(path, 5)
}
```

#### 4.3 스트리밍 메시지 처리

```go
// 기존: 전체 메시지 수신 후 처리
func processMessage(data []byte) {
    msg := json.Unmarshal(data)  // 전체 파싱
    handle(msg)
}

// 개선: 스트리밍 처리
func processMessageStreaming(reader io.Reader) {
    dec := json.NewDecoder(reader)

    // 헤더만 먼저 읽기
    var header MessageHeader
    dec.Decode(&header)

    // 우선순위에 따라 처리
    if header.Priority >= PriorityHigh {
        processUrgent(dec)
    } else {
        queue.Enqueue(dec)  // 배칭
    }
}
```

#### 4.4 연결 품질 모니터링

```go
type PeerQuality struct {
    PeerID     peer.ID
    RTT        time.Duration     // 평균 RTT
    Jitter     time.Duration     // RTT 분산
    PacketLoss float64           // 패킷 손실률
    Bandwidth  float64           // 추정 대역폭
    Score      float64           // 종합 점수 (0-1)
}

func (n *Node) UpdatePeerQuality(p peer.ID) {
    // Ping으로 RTT 측정
    rtt, _ := n.host.Ping(p)

    // 품질 점수 계산
    score := calculateScore(rtt, n.peers[p].packetLoss)

    // 품질 낮은 피어 메시에서 제외
    if score < 0.5 {
        n.gossipsub.RemoveFromMesh(p)
    }
}
```

---

## 5. 신뢰성 및 복원력 개선

### 현재 상태
- GossipSub 확률적 전달 보장
- 메시지 히스토리 없음
- 파티션 복구 메커니즘 없음

### 개선안

#### 5.1 메시지 영속성 (Message Persistence)

```go
type MessageLog struct {
    db        *bbolt.DB
    retention time.Duration // 보존 기간 (e.g., 1시간)
}

func (ml *MessageLog) Append(msg Message) error {
    return ml.db.Update(func(tx *bbolt.Tx) error {
        b := tx.Bucket([]byte("messages"))
        key := []byte(fmt.Sprintf("%d-%s", msg.Timestamp, msg.ID))
        return b.Put(key, msg.Data)
    })
}

// 재연결 시 누락 메시지 동기화
func (ml *MessageLog) GetSince(since time.Time) []Message {
    // 타임스탬프 이후 메시지 반환
}
```

#### 5.2 상태 스냅샷 및 복구

```go
type StateSnapshot struct {
    Timestamp   time.Time
    VectorClock map[string]uint64
    Locks       []Lock
    ContextCIDs []string // 컨텍스트 CID 목록
}

func (a *App) CreateSnapshot() *StateSnapshot {
    return &StateSnapshot{
        Timestamp:   time.Now(),
        VectorClock: a.syncManager.VectorClock(),
        Locks:       a.lockService.ActiveLocks(),
        ContextCIDs: a.vectorStore.AllCIDs(),
    }
}

// 새 피어 참여 시 스냅샷 전송
func (a *App) SendSnapshotToPeer(p peer.ID) error {
    snap := a.CreateSnapshot()
    return a.node.SendDirect(p, "/agent-collab/snapshot", snap)
}
```

#### 5.3 파티션 감지 및 복구

```go
type PartitionDetector struct {
    expectedPeers int
    lastSeen      map[peer.ID]time.Time
    threshold     time.Duration // e.g., 30초
}

func (pd *PartitionDetector) Check() PartitionStatus {
    active := 0
    for _, t := range pd.lastSeen {
        if time.Since(t) < pd.threshold {
            active++
        }
    }

    if active < pd.expectedPeers/2 {
        return PartitionStatus{
            Type:         "minority",
            ActivePeers:  active,
            MissingPeers: pd.expectedPeers - active,
        }
    }
    return PartitionStatus{Type: "healthy"}
}

// 파티션 복구 시
func (a *App) OnPartitionHealed() {
    // 1. 벡터 클럭 비교
    // 2. 누락 델타 교환
    // 3. 락 상태 조정
}
```

---

## 6. 보안 강화

### 현재 상태
- Noise Protocol (전송 암호화)
- Ed25519 (피어 인증)
- 프로젝트별 토픽 분리
- ACL 없음

### 개선안

#### 6.1 토픽 레벨 ACL

```go
type TopicACL struct {
    Topic      string
    AllowList  []peer.ID  // 허용 피어
    DenyList   []peer.ID  // 차단 피어
    PublicKey  []byte     // 토픽 암호화 키
}

// 메시지 암호화
func (acl *TopicACL) Encrypt(msg []byte) ([]byte, error) {
    return aes.Encrypt(acl.PublicKey, msg)
}

// 구독 시 검증
func (acl *TopicACL) CanSubscribe(p peer.ID) bool {
    if slices.Contains(acl.DenyList, p) {
        return false
    }
    if len(acl.AllowList) > 0 {
        return slices.Contains(acl.AllowList, p)
    }
    return true
}
```

#### 6.2 메시지 서명

```go
type SignedMessage struct {
    Payload   []byte
    Signature []byte
    PublicKey []byte
    Timestamp time.Time
}

func (n *Node) SignMessage(msg []byte) *SignedMessage {
    sig, _ := n.privateKey.Sign(msg)
    return &SignedMessage{
        Payload:   msg,
        Signature: sig,
        PublicKey: n.publicKey,
        Timestamp: time.Now(),
    }
}

// 수신 시 검증
func (n *Node) VerifyMessage(sm *SignedMessage) bool {
    // 1. 서명 검증
    if !crypto.Verify(sm.PublicKey, sm.Payload, sm.Signature) {
        return false
    }
    // 2. 타임스탬프 검증 (재전송 공격 방지)
    if time.Since(sm.Timestamp) > 5*time.Minute {
        return false
    }
    return true
}
```

---

## 7. 모니터링 및 관찰성

### 개선안

#### 7.1 메트릭 수집

```go
var (
    messagesSent = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "agentcollab_messages_sent_total",
            Help: "Total messages sent by topic",
        },
        []string{"topic", "type"},
    )

    messagePropagationLatency = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "agentcollab_message_propagation_seconds",
            Help:    "Message propagation latency",
            Buckets: []float64{.01, .025, .05, .1, .25, .5, 1},
        },
        []string{"topic"},
    )

    peerConnections = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "agentcollab_peer_connections",
            Help: "Current number of peer connections",
        },
    )
)
```

#### 7.2 분산 트레이싱

```go
func (a *App) BroadcastContext(ctx context.Context, content string) error {
    span, ctx := opentracing.StartSpanFromContext(ctx, "broadcast_context")
    defer span.Finish()

    span.SetTag("content_size", len(content))

    // 트레이스 ID를 메시지에 포함
    traceID := span.Context().(jaeger.SpanContext).TraceID()
    msg.TraceID = traceID.String()

    return a.node.Publish(ctx, topic, msg)
}
```

---

## 8. 구현 우선순위

### Phase 1: 단기 개선 (1-2주)

| 항목 | 복잡도 | 영향도 | ROI |
|------|--------|--------|-----|
| 메시지 압축 (zstd) | 낮음 | 높음 | ⭐⭐⭐⭐⭐ |
| 메시지 배칭 | 중간 | 높음 | ⭐⭐⭐⭐ |
| GossipSub 프로필 | 낮음 | 중간 | ⭐⭐⭐⭐ |
| 메트릭 수집 | 낮음 | 중간 | ⭐⭐⭐ |

### Phase 2: 중기 개선 (1-2개월)

| 항목 | 복잡도 | 영향도 | ROI |
|------|--------|--------|-----|
| 로컬 우선 락 | 중간 | 높음 | ⭐⭐⭐⭐ |
| 컨텐츠 주소 지정 | 중간 | 높음 | ⭐⭐⭐⭐ |
| 피어 품질 모니터링 | 중간 | 중간 | ⭐⭐⭐ |
| 상태 스냅샷 | 높음 | 중간 | ⭐⭐⭐ |

### Phase 3: 장기 개선 (3-6개월)

| 항목 | 복잡도 | 영향도 | ROI |
|------|--------|--------|-----|
| 계층적 토폴로지 | 높음 | 높음 | ⭐⭐⭐ |
| 지역성 클러스터링 | 높음 | 높음 | ⭐⭐⭐ |
| 토픽 레벨 ACL | 높음 | 중간 | ⭐⭐ |
| 분산 트레이싱 | 중간 | 낮음 | ⭐⭐ |

---

## 9. 예상 성능 개선

### 현재 vs 목표

| 지표 | 현재 | Phase 1 | Phase 2 | Phase 3 |
|------|------|---------|---------|---------|
| 메시지 크기 | 1KB | 0.4KB (-60%) | 0.2KB (-80%) | 0.2KB |
| 전파 지연 | 200ms | 100ms | 50ms | 30ms |
| 처리량 | 100 msg/s | 300 msg/s | 500 msg/s | 1000 msg/s |
| 최대 피어 | 100 | 200 | 500 | 1000+ |
| 락 획득 시간 | 50ms | 20ms | 5ms | 5ms |
| 대역폭 사용 | 100% | 50% | 30% | 25% |

---

## 10. 결론

### 핵심 개선 방향

1. **수직 최적화**: 메시지 압축, 배칭으로 즉시 성능 향상
2. **수평 확장**: 계층적 토폴로지로 대규모 클러스터 지원
3. **사용자 경험**: 로컬 우선 처리로 체감 지연 최소화
4. **운영 가시성**: 메트릭/트레이싱으로 문제 조기 감지

### 다음 단계

1. Phase 1 항목 이슈 생성 및 구현 시작
2. 성능 벤치마크 환경 구축
3. 부하 테스트 시나리오 정의
4. 개선 효과 측정 대시보드 구축
