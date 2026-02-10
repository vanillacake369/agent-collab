# E2E 테스트 가이드

Multipass VM을 사용한 agent-collab 클러스터 E2E 테스트 가이드입니다.

## 사전 요구사항

- macOS (Apple Silicon 또는 Intel)
- [Multipass](https://multipass.run/) 설치: `brew install multipass`
- Go 1.21 이상

## 빠른 시작

```bash
# 전체 테스트 실행 (설정 + 초기화 + 테스트)
./scripts/e2e-test.sh all

# 테스트 완료 후 정리
./scripts/e2e-test.sh cleanup
```

## 단계별 실행

```bash
# 1. VM 설정 (생성 + 패키지 설치 + 바이너리 배포)
./scripts/e2e-test.sh setup

# 2. 클러스터 초기화
./scripts/e2e-test.sh init

# 3. 테스트 실행
./scripts/e2e-test.sh test
```

## 테스트 항목

### 1. 클러스터 연결
- 3개 노드(peer1, peer2, peer3) P2P 연결 확인
- GossipSub 토픽 구독 확인

### 2. 컨텍스트 공유
- peer1에서 `share_context` 호출
- peer2, peer3에서 `search_similar`로 검색 가능 여부 확인
- P2P 브로드캐스트 동작 검증

### 3. 이벤트 시스템
- `get_events`로 이벤트 히스토리 조회
- `context.updated` 이벤트 기록 확인

## 수동 테스트

### VM 접속
```bash
multipass shell peer1
```

### MCP 도구 직접 호출
```bash
# 컨텍스트 공유
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"share_context","arguments":{"file_path":"test.go","content":"Test content"}}}' | /home/ubuntu/agent-collab mcp serve

# 검색
echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"search_similar","arguments":{"query":"test","limit":5}}}' | /home/ubuntu/agent-collab mcp serve

# 이벤트 조회
echo '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_events","arguments":{"limit":10}}}' | /home/ubuntu/agent-collab mcp serve
```

### 클러스터 상태 확인
```bash
/home/ubuntu/agent-collab daemon status
```

## 로컬 머신에서 클러스터 참여

로컬 Mac에서 VM 클러스터에 참여하여 테스트:

```bash
# 1. 바이너리 빌드
go build -o agent-collab ./cmd/agent-collab

# 2. 데몬 시작
./agent-collab daemon start

# 3. peer1의 정보 확인
PEER1_IP=$(multipass info peer1 | grep IPv4 | awk '{print $2}')
PEER1_PORT=$(multipass exec peer1 -- ss -tlnp | grep agent-collab | grep -oE ":[0-9]+" | head -1 | tr -d ':')
PEER1_ID=$(multipass exec peer1 -- /home/ubuntu/agent-collab daemon status | grep "Node ID" | awk '{print $NF}')

# 4. 토큰 생성 및 참여
TOKEN=$(echo -n '{"addrs":["/ip4/'$PEER1_IP'/tcp/'$PEER1_PORT'"],"project":"e2e-test","creator":"'$PEER1_ID'","created":'$(date +%s)',"expires":'$(($(date +%s)+86400))'}' | base64)
./agent-collab join "$TOKEN"

# 5. 상태 확인
./agent-collab daemon status
```

## Claude Code 연동

MCP 설정 파일 (`~/.claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "agent-collab": {
      "command": "/path/to/agent-collab",
      "args": ["mcp", "serve"]
    }
  }
}
```

## 문제 해결

### P2P 연결 실패
```bash
# VM 간 네트워크 확인
multipass exec peer1 -- ping -c 1 $(multipass info peer2 | grep IPv4 | awk '{print $2}')

# 방화벽 확인
multipass exec peer1 -- sudo ufw status
```

### 데몬 로그 확인
```bash
multipass exec peer1 -- journalctl -u agent-collab -f
# 또는
multipass exec peer1 -- cat ~/.agent-collab/daemon.log
```

## 정리

```bash
# VM 삭제
./scripts/e2e-test.sh cleanup

# 또는 수동 삭제
multipass delete peer1 peer2 peer3 --purge
```
