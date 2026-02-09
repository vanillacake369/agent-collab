# Phase 2: Researcher Report - 기술 조사 및 베스트 프랙티스

## 1. Multipass VM 네트워킹 조사

### 1.1 macOS에서 Bridged 네트워크 설정

**참고**: [Multipass Networks Documentation](https://documentation.ubuntu.com/multipass/latest/reference/command-line-interface/networks/)

#### 기본 명령어

```bash
# 사용 가능한 네트워크 확인
multipass networks

# 브릿지 네트워크 설정
multipass set local.bridged-network=en0

# 브릿지 네트워크로 VM 생성
multipass launch -n peer1 --network en0
```

#### VirtualBox 드라이버 사용 (권장)

Apple Silicon Mac에서 QEMU의 브릿지 네트워킹 제한으로 인해 VirtualBox 드라이버가 더 안정적:

```bash
# VirtualBox 드라이버 설정
multipass set local.driver=virtualbox

# 브릿지 네트워크로 VM 생성
multipass launch -n peer1

# VBoxManage로 브릿지 설정
VBoxManage modifyvm peer1 --nic2 bridged --bridgeadapter2 en0
```

### 1.2 NAT 모드 대안 (Bootstrap 방식)

기본 NAT 모드에서도 libp2p의 Bootstrap peer를 통해 연결 가능:

```bash
# NAT 모드 VM들 생성 (기본)
multipass launch -n peer1
multipass launch -n peer2
multipass launch -n peer3

# peer1에서 init → 토큰에 bootstrap 주소 포함
# peer2, peer3에서 토큰 사용하여 join
```

### 1.3 알려진 이슈

- **macOS Tahoe**: bridge100 인터페이스가 192.168.2.1을 사용하여 로컬 네트워크와 충돌 가능
- **Apple Silicon + QEMU**: en0 이외의 브릿지 인터페이스 미지원

## 2. libp2p NAT Traversal 조사

### 2.1 프로덕션 성공률 벤치마크

**참고**: [libp2p Hole Punching](https://docs.libp2p.io/concepts/nat/hole-punching/), [DCUtR Protocol](https://docs.libp2p.io/concepts/nat/dcutr/)

2025년 대규모 측정 연구 결과 (IPFS 네트워크):
- **4.4M+ 시도**, 85,000+ 네트워크, 167개국
- **Hole Punching 성공률: ~70% ± 7.1%**
- **TCP ≈ QUIC ≈ 70%** (UDP 우위설 반박됨)

### 2.2 DCUtR (Direct Connection Upgrade through Relay)

agent-collab에 이미 구현된 기능:
- ✅ AutoNAT v2 (`libp2p.EnableAutoNATv2()`)
- ✅ Hole Punching (`libp2p.EnableHolePunching()`)
- ✅ Relay Service (`libp2p.EnableRelayService()`)
- ✅ NAT Port Map (`libp2p.NATPortMap()`)

### 2.3 재시도 전략 베스트 프랙티스

```
재시도 타이밍: 0, 250ms, 500ms, 1s, 2s (총 5회)
전체 타임아웃: ~4초
```

### 2.4 Fallback 메커니즘

```
1차: Direct Connection (hole punching)
2차: Circuit Relay (중계 노드 경유)
3차: Alternative Transport (QUIC/UDP)
```

### 2.5 모니터링

libp2p 공식 Grafana 대시보드:
https://github.com/libp2p/go-libp2p/blob/master/dashboards/holepunch/holepunch.json

## 3. Claude Code MCP 통합 베스트 프랙티스

### 3.1 설정 범위 (Scope)

**참고**: [Claude Code MCP Docs](https://code.claude.com/docs/en/mcp)

| 범위 | 저장 위치 | 용도 |
|-----|----------|------|
| Local | `~/.claude.json` (프로젝트별) | 개인 개발 서버, 민감한 자격증명 |
| Project | `.claude/mcp.json` | 팀 공유, 버전 관리 |
| Global | `~/.claude/mcp.json` | 모든 프로젝트 공통 |

### 3.2 보안 베스트 프랙티스

1. **최소 권한 원칙**: 필요한 접근만 부여 (read-only vs write)
2. **TLS/HTTPS**: 모든 외부 서비스 통신 암호화
3. **활동 로깅**: 민감 정보는 평문 저장 금지
4. **신뢰할 수 있는 서버만 설치**: Prompt injection 위험 주의

### 3.3 효율성 베스트 프랙티스

**참고**: [Code Execution with MCP](https://www.anthropic.com/engineering/code-execution-with-mcp)

수백~수천 개의 도구를 사용할 때:
- 모든 도구 정의를 미리 로드하면 속도 저하 및 비용 증가
- **온디맨드 로딩**: 필요할 때만 도구 로드
- **데이터 필터링**: 모델에 전달 전 데이터 정제
- **단일 스텝 실행**: 복잡한 로직을 한 번에 처리

### 3.4 MCP 서버 설정 예시 (agent-collab)

```json
{
  "mcpServers": {
    "agent-collab": {
      "command": "/usr/local/bin/agent-collab",
      "args": ["mcp", "serve"],
      "env": {
        "AGENT_COLLAB_EMBEDDING_PROVIDER": "openai",
        "OPENAI_API_KEY": "${OPENAI_API_KEY}"
      }
    }
  }
}
```

### 3.5 Transport 옵션

| 타입 | 사용 사례 | 장점 |
|-----|----------|------|
| stdio | 로컬 프로세스 | 간단, 빠름 |
| HTTP | 원격 서버 | 클라우드 서비스 연동 |
| SSE | 이벤트 스트리밍 | 실시간 알림 |

## 4. 경쟁 솔루션 벤치마킹

### 4.1 Continue.dev

**참고**: [Continue Agent Docs](https://docs.continue.dev/agent/how-to-use-it)

- **아키텍처**: IDE 확장 (VS Code, JetBrains)
- **컨텍스트 공유**: @ context providers, MCP 도구 지원
- **멀티 모델**: OpenAI, Anthropic, Ollama, 커스텀 모델
- **Agent Mode**: 도구를 자동으로 선택하여 작업 수행

**장점**: IDE 통합 UX, 모델 선택 자유
**한계**: 단일 세션 내에서만 동작, P2P 컨텍스트 공유 없음

### 4.2 Aider

**참고**: [Aider Alternatives](https://replit.com/discover/aider-alternative)

- **아키텍처**: CLI 기반, 로컬 저장소 접근
- **컨텍스트 공유**: 명시적 파일 지정, multi-file 변경
- **프라이버시**: 로컬 실행, self-hosted 모델 지원

**장점**: 오픈소스, 저장소 직접 수정 가능
**한계**: 단일 사용자, 팀 협업 기능 없음

### 4.3 Cline

**참고**: [Cline Blog](https://cline.bot/blog/12-coding-agents-defining-the-future-of-ai-development)

- **아키텍처**: VS Code 확장
- **특징**: 완전 오픈소스, 모델 무관

### 4.4 agent-collab의 차별점

| 기능 | Continue | Aider | Cline | agent-collab |
|-----|----------|-------|-------|--------------|
| P2P 컨텍스트 공유 | ❌ | ❌ | ❌ | ✅ |
| 분산 Lock 관리 | ❌ | ❌ | ❌ | ✅ |
| 멀티 에이전트 협업 | ❌ | ❌ | ❌ | ✅ |
| 벡터 기반 검색 | ❌ | ❌ | ❌ | ✅ |
| 충돌 감지/해결 | Git 의존 | Git 의존 | Git 의존 | CRDT 기반 |
| MCP 지원 | ✅ | ❌ | ❌ | ✅ |

## 5. Context Engineering 트렌드 (2025)

**참고**: [Google Developers Blog](https://developers.googleblog.com/architecting-efficient-context-aware-multi-agent-framework-for-production/)

### 5.1 문제점

에이전트 실행이 길어질수록 추적할 정보 폭발:
- 채팅 기록
- 도구 출력
- 외부 문서
- 중간 추론

### 5.2 해결 접근법

단순히 더 큰 컨텍스트 윈도우에 의존하는 것이 아닌, **Context Engineering** 필요:
- 컨텍스트를 독립적인 시스템으로 취급
- 자체 아키텍처, 수명주기, 제약조건 정의

### 5.3 agent-collab의 접근법

```
1. Vector Embedding: 의미론적 컨텍스트 압축
2. CRDT Delta Log: 변경 이력만 동기화
3. Gossipsub: 필요한 피어에게만 전파
4. Semantic Lock: 관련 컨텍스트 영역만 잠금
```

## 6. 테스트 인프라 권장사항

### 6.1 단위 테스트

- 기존 Go 테스트 프레임워크 활용
- Mock 네트워크 (tailscale natlab 참고)

### 6.2 통합 테스트

- Docker Compose: 빠른 반복
- 네트워크 격리: Docker network 사용

### 6.3 E2E 테스트

- Multipass VM: 실제 OS 환경
- Bridged 네트워크: mDNS 발견 테스트
- NAT 모드: Bootstrap 방식 테스트

### 6.4 CI/CD 통합

```yaml
# GitHub Actions 예시
jobs:
  e2e-test:
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install Multipass
        run: brew install multipass
      - name: Run E2E Tests
        run: make test-e2e-multipass
```

## 7. 다음 단계 (Phase 3: Planning)

조사 결과를 바탕으로 계획해야 할 항목:
1. Multipass NAT 모드 + Bootstrap 방식 채택 (브릿지 불안정)
2. libp2p 70% 성공률 고려한 fallback 전략
3. MCP stdio 전송 방식 유지 (각 VM 로컬 실행)
4. 3-VM 테스트 시나리오 구체화
5. 자동화 스크립트 설계
