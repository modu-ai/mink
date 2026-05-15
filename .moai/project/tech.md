# MINK - Technology Document v4.1 POLYGLOT HYBRID

> **비전:** 글로벌 오픈소스 AI 에이전트 - Polyglot 3-언어 하이브리드 아키텍처
> **핵심:** Rust (크리티컬 20%) + Go (오케스트레이션 70%) + TypeScript (UI 10%)
> **기반:** MoAI-ADK-Go (38,700줄) 직접 계승 + 2026 업계 표준 (Cloudflare, Mullvad, Microsoft)

---

## 0. 아키텍처 철학 (2026 Industry Consensus)

### 0.1 "Rust vs Go" 논쟁은 끝났다

2026년 업계 합의: **하이브리드**가 답.

- **Rust**: 성능 크리티컬 "engine" 레이어 (hot paths, 보안, ML)
- **Go**: 오케스트레이션 "business logic" 레이어 (동시성, IO)
- **TypeScript**: 엣지/클라이언트 레이어 (UI, SDK)

### 0.2 검증된 2026 패턴

| 회사/프로젝트 | 사례 | 교훈 |
|-------------|------|------|
| **Cloudflare Pingora** | Rust 프록시 | 성능 크리티컬 = Rust |
| **AWS Firecracker** | Rust VM | 보안 = Rust |
| **Microsoft Wassette** | Rust + WASM + MCP | AI 에이전트 + WASM = Rust |
| **Mullvad GotaTun** | Go → Rust WireGuard | E2EE = Rust |
| **Agentor** | 13 Rust 크레이트 + WASM | <50ms cold start |
| **Eino (ByteDance)** | Go AI 오케스트레이션 | 고동시성 = Go |
| **Google ADK** | Go 에이전트 SDK | 에이전트 협업 = Go |
| **$50K → $35K/월** | Hot path 3개만 Rust | 30% 비용 절감 |

### 0.3 MINK의 선택 근거

- **MoAI-ADK-Go 38,700줄 직접 재사용** → Go 오케스트레이션 70%
- **2026 벤치마크**: Rust 2-12x 빠른 CPU, 40% 낮은 레이턴시
- **보안 크리티컬 영역**: E2EE (GotaTun 패턴), WASM (Wassette 패턴), ML (Candle/Burn)
- **개발자 경험**: TypeScript UI 생태계 최대

---

## 1. 기술 스택 개요 (Polyglot Hybrid)

### 1.1 3-언어 6-계층 아키텍처

```
┌──────────────────────────────────────────────────────────────┐
│  📘 TypeScript Layer (Client Edge - 10% LOC)                 │
│  mink-cli (Ink) │ mink-desktop (Tauri) │                     │
│  mink-mobile (RN) │ mink-web (Next.js)                       │
├──────────────────────────────────────────────────────────────┤
│  Bridge: gRPC (.proto contracts, cross-language)             │
├──────────────────────────────────────────────────────────────┤
│  🐹 Go Layer (Orchestration Majority - 70% LOC)              │
│  minkd daemon │ Agent Runtime │ Learning Coord               │
│  LLM Router │ Tools │ Memory │ Transport │ Gateway           │
├──────────────────────────────────────────────────────────────┤
│  Bridge: gRPC + CGO (hot paths only)                         │
├──────────────────────────────────────────────────────────────┤
│  🦀 Rust Layer (Performance & Security Critical - 20% LOC)   │
│  mink-ml (LoRA 훈련, Vector 연산)                             │
│  mink-wasm (Extism/Wasmtime 샌드박스)                         │
│  mink-crypto (E2EE Relay, Noise Protocol)                    │
│  mink-desktop (OS Accessibility API)                         │
│  mink-graph (Identity Graph 최적화)                           │
├──────────────────────────────────────────────────────────────┤
│  Infrastructure                                               │
│  SQLite/FTS5 │ Qdrant │ Graphiti │ Ollama │ WASI            │
└──────────────────────────────────────────────────────────────┘
```

### 1.2 레이어별 언어 매핑 (최종 결정)

| # | 레이어 | 언어 | 근거 | 참조 |
|---|-------|-----|------|------|
| 1 | **Daemon 오케스트레이터** (minkd) | 🐹 **Go** | MoAI-ADK-Go 계승, goroutines | MoAI-ADK |
| 2 | **Agent Runtime** | 🐹 Go | Google ADK, goroutines | ADK-Go |
| 3 | **Learning Engine 코어** | 🐹 Go | SPEC-REFLECT 계승 | MoAI-ADK |
| 4 | **ML/LoRA 훈련 엔진** | 🦀 **Rust** | Candle/Burn, SIMD, GPU | Agentor |
| 5 | **Knowledge Graph** | 🐹 Go | Graphiti-go | Zep |
| 6 | **Vector 유사도 계산** | 🦀 Rust | hnsw-rs, SIMD | - |
| 7 | **LLM Router** | 🐹 Go | IO 바운드, Eino | ByteDance |
| 8 | **Tool Execution** | 🐹 Go | MoAI 계승 | - |
| 9 | **WASM Sandbox** | 🦀 **Rust** | Wasmtime 네이티브 | Wassette |
| 10 | **E2EE Relay/암호화** | 🦀 **Rust** | Mullvad GotaTun | wireguard-rs |
| 11 | **Desktop Automation** | 🦀 Rust | accessibility-ng | OculOS |
| 12 | **Memory/Storage** | 🐹 Go | sqlx | Hermes |
| 13 | **Transport** (gRPC/WS) | 🐹 Go | tonic도 가능 but 일관성 | - |
| 14 | **CLI Terminal UI** | 📘 **TS** | Ink/React | Claude Code |
| 15 | **Desktop App** | 📘 TS + Tauri(Rust) | Tauri v2 | - |
| 16 | **Mobile App** | 📘 TS | React Native | - |
| 17 | **Web Client** | 📘 TS | Next.js 15 | - |
| 18 | **Plugins** | Any→WASM | Extism PDK | - |

### 1.3 LOC 분포 예상 (Year 2 기준)

| 언어 | 예상 LOC | 비율 | 역할 |
|-----|---------|------|------|
| **Go** | ~60,000 | 70% | 오케스트레이션 |
| **Rust** | ~15,000 | 20% | 크리티컬 |
| **TypeScript** | ~8,000 | 10% | UI |
| **Total** | ~83,000 | 100% | - |

---

## 2. Rust 기술 스택 (Critical Layer - 20%)

### 2.1 Rust 적용 영역 (5개 크레이트)

**mink-ml** (ML/LoRA 훈련):
| 의존성 | 용도 |
|-------|------|
| **candle-core** | Huggingface의 Rust ML 프레임워크 |
| **burn** | Pure Rust ML 프레임워크 (GPU 지원) |
| **tch-rs** | PyTorch Rust 바인딩 |
| **ort** | ONNX Runtime Rust |
| **safetensors** | 안전한 tensor 직렬화 |
| **ndarray** | NumPy-like 배열 |
| **rayon** | 데이터 병렬 처리 |

**mink-wasm** (WASM 샌드박스):
| 의존성 | 용도 |
|-------|------|
| **wasmtime** | Microsoft Wassette 기반 |
| **extism** | 다국어 플러그인 SDK |
| **wit-bindgen** | Component Model |
| **cap-std** | Capability-based std |

**mink-crypto** (E2EE & Relay):
| 의존성 | 용도 |
|-------|------|
| **snow** | Noise Protocol (WireGuard 동일) |
| **boring** | BoringSSL 래퍼 (Cloudflare) |
| **ring** | 암호 primitives |
| **x25519-dalek** | X25519 KEM |
| **chacha20poly1305** | AEAD |
| **argon2** | 패스워드 해싱 |

**mink-desktop** (OS Accessibility):
| 의존성 | 용도 |
|-------|------|
| **accessibility-ng** | macOS AXUIElement |
| **screencapturekit-rs** | macOS 스크린 |
| **enigo** | 크로스 플랫폼 입력 |
| **core-graphics** | macOS CG |
| **windows-rs** | Windows UIA |
| **atspi** | Linux AT-SPI2 |

**mink-vector** (Vector 연산):
| 의존성 | 용도 |
|-------|------|
| **hnsw_rs** | HNSW 근사 최근접 |
| **qdrant-client** | Qdrant Rust |
| **simba** | SIMD 선형대수 |
| **nalgebra** | 수치 연산 |

### 2.2 Rust 빌드 & 통합

**Cargo workspace**:
```toml
[workspace]
members = [
    "crates/mink-ml",
    "crates/mink-wasm",
    "crates/mink-crypto",
    "crates/mink-desktop",
    "crates/mink-vector",
]
```

**Go ↔ Rust 통신**:
- **기본**: gRPC (tonic ↔ grpc-go)
  - 네트워크 오버헤드 ~1-2ms (무시할만)
  - 독립 배포 가능
  - Protobuf 스키마 공유
- **핫 패스만**: CGO + Rust FFI
  - 네트워크 오버헤드 0
  - 동일 프로세스
  - 복잡도 증가

---

## 3. Go 기술 스택 (Orchestration - 70%)

### 3.1 런타임 & 코어

| 의존성 | 버전 | 용도 |
|-------|------|------|
| **Go** | 1.26+ | 언어 (MoAI-ADK-Go 동일) |
| **tokio** (없음) | - | Go는 goroutines 내장 |
| stdlib `context` | - | 취소/타임아웃 |
| stdlib `sync` | - | 동시성 |
| **uber-go/zap** | 1.27+ | 로깅 |
| spf13/viper | 1.19+ | Config |
| spf13/cobra | 1.8+ | CLI |

### 3.2 AI / LLM (Go 오케스트레이션)

| 의존성 | 용도 |
|-------|------|
| **github.com/cloudwego/eino** | ByteDance LLM 프레임워크 |
| github.com/sashabaranov/go-openai | OpenAI 호환 |
| github.com/anthropic/anthropic-sdk-go | Claude |
| github.com/ollama/ollama/api | Ollama |
| **github.com/firebase/genkit-go** | Google Genkit |
| **github.com/google/adk-go** | Agent Developer Kit |
| github.com/tiktoken-go/tokenizer | 토큰 카운팅 |

### 3.3 통신 & 프로토콜

| 의존성 | 용도 |
|-------|------|
| google.golang.org/grpc | gRPC |
| google.golang.org/protobuf | Protobuf |
| **github.com/modelcontextprotocol/go-sdk** | MCP |
| **github.com/a2aproject/a2a-go** | A2A v0.3 |
| github.com/gorilla/websocket | WebSocket |
| github.com/gin-gonic/gin | HTTP |

### 3.4 Rust FFI 통합 (hot paths only)

| 의존성 | 용도 |
|-------|------|
| **cgo** (stdlib) | C 인터페이스 |
| **purego** | Pure Go dlopen (CGO 없이) |
| `crates/mink-ml/mink_ml.h` | Rust → C 헤더 |

### 3.5 데이터베이스

| 의존성 | 용도 |
|-------|------|
| github.com/mattn/go-sqlite3 | SQLite FTS5 |
| **github.com/getzep/graphiti-go** | Identity Graph |
| github.com/kuzu-db/kuzu-go | Embedded graph |

---

## 3.6 Design 생산 스택 (신규: SPEC-AGENCY-ABSORB-001)

| 기술 | 용도 | 상태 |
|-----|------|------|
| **Pencil MCP** | 디자인 파일 (.pen) 읽기/쓰기 | Active |
| **Claude Design** | AI-생성 디자인 번들 임포트 (Path A) | Supported |
| **moai-domain-brand-design** | 브랜드 토큰·컴포넌트 spec 생성 | Active |
| **moai-domain-copywriting** | 브랜드 음성·톤·마케팅 카피 | Active |
| **moai-workflow-gan-loop** | Builder-Evaluator 반복 개선 루프 | Active |
| **moai-workflow-pencil-integration** | .pen 파일 로드·검증 | Active |
| **Mermaid** | 아키텍처 다이어그램 | Embedded |

**워크플로우**: manager-spec → (moai-domain-copywriting + moai-domain-brand-design) 병렬 → expert-frontend → evaluator-active (GAN Loop)

---

## 3.7 데이터베이스 메타 관리 스택 (신규: SPEC-DB-SYNC-RELOC-001)

| 기술 | 용도 | 파서 |
|-----|------|------|
| **Prisma** | ORM + 마이그레이션 | `.prisma` 파일 파싱 |
| **Alembic** | Python 마이그레이션 | `versions/*.py` 파싱 |
| **Rails ActiveRecord** | Ruby 마이그레이션 | `migrate/*.rb` 파싱 |
| **Supabase** | PostgreSQL + RLS 정책 | API 통합 |
| **Mermaid ERD** | Entity-Relationship Diagram | 자동 생성 |
| **moai-domain-db-docs** | 스키마·쿼리·정책 문서 생성 | `.md` 출력 |

**산출물**: `.moai/project/db/` 디렉터리에 schema.md, migrations.md, erd.mmd, rls-policies.md, queries.md, seed-data.md 자동 생성

**참조**: `/moai db` 명령어로 동기화

---

## 4. TypeScript 기술 스택 (Client UI - 10%)

### 4.1 mink-cli (Terminal)
- ink 6.x (ESM), react 19.x, @inkjs/ui 3.x, commander 12.x
- @connectrpc/connect 2.x (gRPC 클라이언트)

### 4.2 mink-desktop (Tauri v2)
- tauri 2.x (Rust backend 자체적)
- react 19.x, zustand 5.x, tailwindcss 4.x
- shadcn/ui, framer-motion 11.x

### 4.3 mink-mobile (React Native)
- react-native 0.76+ (New Architecture)
- @picovoice/porcupine-react-native 3.x
- whisper.rn 0.4+
- react-native-executorch 0.2+ (LoRA)
- react-native-stripe

### 4.4 mink-web (Next.js 15)
- next 15.x, react 19.x
- @connectrpc/connect-web 2.x

---

## 5. 빌드 시스템 (3-언어)

### 5.1 통합 빌드 파이프라인

```bash
# 1. Rust 크레이트 빌드 (mink-ml, mink-wasm 등)
cd crates
cargo build --release
cbindgen --crate mink-ml --output mink_ml.h  # C 헤더 생성

# 2. Go 바이너리 빌드 (Rust 의존성 link)
cd ..
go build -o bin/minkd ./cmd/minkd/
go build -o bin/mink ./cmd/mink/

# 3. TypeScript 빌드 (Turborepo)
cd packages
turbo build

# 4. Tauri 데스크톱 (Rust + TS)
cd packages/mink-desktop
bun tauri build
```

### 5.2 크로스 컴파일 (10 targets)

- **Desktop**: 5 (macOS x86/arm, Linux x86/arm, Windows)
- **Mobile**: 2 (iOS, Android)
- 각 타깃별 Rust + Go + TS 병합

### 5.3 CI/CD 매트릭스

```yaml
jobs:
  rust-check:
    - cargo fmt, clippy, test
  go-check:
    - go fmt, vet, golangci-lint, test
  ts-check:
    - turbo lint, typecheck, test
  integration:
    - Rust ↔ Go FFI 테스트
    - gRPC 프로토 버전 호환
  cross-build:
    - 10 platforms matrix
```

---

## 6. 보안 모델 (Rust 크리티컬 영역)

### 6.1 E2EE (Rust mink-crypto)

**Noise Protocol + WireGuard 영감**:
- X25519 (키 교환)
- ChaCha20-Poly1305 (AEAD)
- Blake2s (MAC)
- Forward secrecy
- Post-quantum preshared key 옵션

**Mullvad GotaTun 패턴**:
- Go → Rust 마이그레이션 이유
- 메모리 제어 + 비밀 누출 방지
- 배터리 수명 향상

### 6.2 WASM Sandbox (Rust mink-wasm)

**Wasmtime 기반 (Microsoft Wassette 패턴)**:
- 메모리 격리
- 콜스택 접근 불가
- 메모리 제로화 (누수 방지)
- Capability-based 권한

**Extism 다국어 플러그인**:
- Rust, Go, C, AssemblyScript, Zig 지원
- OCI 레지스트리 + OSV 스캔

### 6.3 OS Accessibility (Rust mink-desktop)

**네이티브 OS API 안전 접근**:
- macOS: AXUIElement, ScreenCaptureKit
- Windows: UIA (UI Automation)
- Linux: AT-SPI2 D-Bus
- Rust FFI 안전성

### 6.4 10-Layer Security (그대로 유지)

Rust 크리티컬 영역이 L4 (WASM), L7 (E2EE)을 강화:
- L1: User Approval (Go)
- L2: Agent Permission (Go)
- L3: Token Budget (Go)
- **L4: WASM Sandbox (Rust)** ⭐
- L5: Zone Isolation (Go)
- L6: Network Isolation (Go)
- **L7: E2E Encryption (Rust)** ⭐
- L8: Audit Log (Go)
- L9: Behavioral Trust (Go)
- L10: OWASP Top 10 (Go + Rust hybrid)

---

## 7. 성능 목표 (하이브리드 기대 효과)

| 메트릭 | v4.0 Go Only | **v4.1 Polyglot** | 개선 |
|--------|-------------|------------------|------|
| 콜드 스타트 | < 300ms | **< 200ms** | 33% ↓ |
| 에이전트 루프 | < 40ms | < 40ms | 동일 |
| 메모리 (idle) | < 40MB | **< 30MB** | 25% ↓ |
| 동시 에이전트 | 1000+ | 1000+ | 동일 |
| **LoRA 추론** | 150ms (Ollama) | **< 50ms (Rust)** | 3x ↑ |
| **Vector 유사도 1M** | 200ms | **< 20ms (SIMD)** | 10x ↑ |
| **E2EE Relay 처리량** | 800 Mbps | **> 2 Gbps** | 2.5x ↑ |
| **WASM 콜드 스타트** | 150ms | **< 50ms** | 3x ↑ |
| LLM Router 레이턴시 | < 10ms | < 10ms | 동일 |

**절감 효과**:
- 인프라 비용: 20-30% (Rust hot path)
- 배터리 (모바일): 15-20% (Mullvad 관찰)

---

## 8. 아키텍처 결정 기록 (16 ADR, v4.1 업데이트)

### ADR-001: Polyglot 3-Language Hybrid (v4.1 핵심 변경) ⭐

**Decision:** Rust (크리티컬 20%) + Go (오케스트레이션 70%) + TypeScript (UI 10%)

**Rationale:**
- **MoAI-ADK-Go 38,700줄 직접 계승** (Go 70%)
- 2026 업계 표준 (Cloudflare, AWS, Microsoft, Mullvad)
- 핫 패스/보안만 Rust → 2-12x 성능 향상 (사례 사례)
- TypeScript UI 생태계 최대 활용
- 30% 인프라 비용 절감 가능 ($50K→$35K 사례)

**Trade-off:**
- 3-언어 CI/CD 복잡도 증가
- Rust ↔ Go FFI 학습 필요
- 그러나 각 레이어에서 최적 = 순증익

### ADR-002: gRPC Client-Server (유지)

**Decision:** gRPC로 모든 언어 경계 통신

### ADR-003: 3-DB 하이브리드 저장소 (유지)

### ADR-004: Inventory-based Tool Auto-Registration (Go)

### ADR-005: Extism WASM 플러그인 (Rust)

**Update:** Wasmtime을 Rust에서 직접 사용 (Wassette 패턴).

### ADR-006: gRPC + WebSocket + E2EE Relay (유지)

### ADR-007: A2A Protocol v0.3 (ADK-Go) (유지)

### ADR-008: Dual TUI (Ink + Bubbletea) (유지)

### ADR-009: Agent-First Architecture (MoAI 계승) (유지)

### ADR-010: OAuth 2.1 + Subscription (유지)

### ADR-011: Self-Evolving Learning Engine

**Update:** 코어 로직은 Go (MoAI 계승), ML/LoRA 훈련은 Rust.

### ADR-012: User-specific LoRA Adapters

**Update:** LoRA 훈련 = **Rust mink-ml**. On-device QLoRA 성능 극대화.

### ADR-013: Privacy-preserving Stack

**Update:** E2EE Relay = **Rust mink-crypto** (Mullvad GotaTun 패턴).

### ADR-014: Identity Graph with POLE+O (유지)

**Note:** Graph 엔진은 Go (Graphiti-go), 핫 쿼리만 Rust SIMD 고려.

### ADR-015: Rust for Security-Critical (v4.1 신규)

**Decision:** 보안 크리티컬 영역은 무조건 Rust:
- WASM 샌드박스 (L4)
- E2EE 암호화 (L7)
- Desktop OS API
- Cryptographic primitives

**Rationale:**
- 메모리 안전 필수
- Mullvad, Wassette, Cloudflare 검증
- Go GC에서 "secret zeroing" 불확실
- Rust `zeroize` 크레이트로 보장

### ADR-016: Rust for ML Hot Paths (v4.1 신규)

**Decision:** ML/Vector 연산 hot paths = Rust

**Rationale:**
- SIMD 활용 (simba, nalgebra)
- Candle/Burn 성숙 (Huggingface)
- GPU 바인딩 (Rust-CUDA)
- Go GC 예측 불가 레이턴시 회피

**Trade-off:** 복잡도 증가 but 10x 성능 향상.

### ADR-017: Zero-Knowledge Credential Pool (v4.1 신규, CREDPOOL-001)

**Decision:** 크레덴셜 저장소는 Zero-Knowledge proof 기반 암호화 저장소 사용

**Rationale:**
- API 키/토큰은 절대 메모리에 평문 보관 금지
- ZK proof로 "이 크레덴셜은 유효함"을 증명하되 값은 encrypted vault에만
- Vault rotation: 매 사용마다 새로운 ephemeral key 생성
- Audit 로그: 누가 언제 어떤 provider credential 접근했는지 추적

**Tradeoff:**
- 암호화/복호화 오버헤드: ~1-2ms (무시할만)
- Provider 폐기시 매우 간단 (vault key 폐기만)

### ADR-018: 4-Bucket Rate Limiter (RATELIMIT-001)

**Decision:** 4개 독립 bucket으로 RPM, TPM, RPH, TPH 각각 추적

**Architecture:**
```
Bucket 1: RPM (Request Per Minute) — Anthropic/OpenAI RPM 제한
Bucket 2: TPM (Token Per Minute) — 모든 provider TPM 제한  
Bucket 3: RPH (Request Per Hour) — Groq 무료 30 RPM/1440 RPH
Bucket 4: TPH (Token Per Hour) — 일일 할당량 제한
```

**Header Parsing:**
- Anthropic: `anthropic-ratelimit-remaining-requests`, `anthropic-ratelimit-remaining-tokens`
- OpenAI: `x-ratelimit-remaining-requests`, `x-ratelimit-remaining-tokens`
- Google: `x-ratelimit-remaining` (TPM only)

**경고 및 차단:**
- 80% 도달 시 경고 (로그 + 사용자 알림)
- 100% 도달 시 차단 + fallback provider로 라우팅
- 메트릭 수집: Prometheus export for alerting

**Trade-off:** 4개 bucket 동시 추적은 최소 오버헤드 (<100ns per call)

### ADR-019: MCP 3-Transport Strategy (MCP-001)

**Decision:** Model Context Protocol 지원 3가지 transport

**Architectures:**
```
1. Stdio: 로컬 프로세스 통신 (클라이언트용)
   - 보안: 자신의 프로세스 범위 내
   - 지연: 0ms (IPC 오버헤드만)
   
2. WebSocket: 실시간 양방향 (서버용)
   - 보안: TLS 필수
   - 지연: 10-50ms (네트워크)
   
3. SSE (Server-Sent Events): 단방향 스트림 + 양방향 HTTP
   - 보안: TLS 필수
   - 지연: 50-100ms (Long-polling 효과)
   - 사용: firewall 뒤 클라이언트 통신
```

**OAuth 2.1 협상:**
- Capability claim in OAuth token
- scope: "mcp:*" (모든 MCP 리소스 접근)
- refresh_token으로 세션 유지

**Trade-off:** 각 transport 별 다른 latency trade-off

### ADR-020: Sub-agent 3-Isolation Strategy (SUBAGENT-001)

**Decision:** 3가지 isolation 모드 지원: fork, worktree, background

**Matrix:**
```
            fork              worktree         background
Process     별도 프로세스     git worktree     비동기 태스크
Memory      독립적           공유 파일시스템  메인 메모리
Return      wait()           git clean        fire-and-forget
Use case    무거운 작업      SPEC 개발        빠른 피드백
Scope       none/session     session/persist  none
```

**Memory Scope (3개):**
- `none`: 메모리 격리 (상호작용 로깅 없음)
- `session`: 세션 메모리만 (이번 실행 맥락)
- `persistent`: .claude/agent-memory/ 접근 가능

**PlanModeApprove:**
- Sub-agent가 plan mode로 생성되면 모든 쓰기 차단
- 읽기만 허용 (codebase exploration)

**Trade-off:**
- fork: 가장 무겁지만 메모리 완전 격리
- worktree: 중간 수준, 파일 편집 완벽 격리
- background: 가장 빠르지만 메인 메모리 영향

### ADR-021: Plugin Host Atomic ClearThenRegister (PLUGIN-001)

**Decision:** Plugin registry는 원자적 clear-then-register 패턴

**Architecture:**
```go
func RegisterPlugin(manifest *PluginManifest) error {
    // 1. 새 plugin 로드 및 검증
    plugin, err := loadPlugin(manifest.Path)
    if err != nil {
        return err
    }
    
    // 2. 기존 모든 plugin 일괄 언로드
    registry.ClearAll()
    
    // 3. 새 plugin 등록
    registry.Register(plugin)
    
    // 4. 의존 plugin들 재등록 (topological order)
    for _, dep := range dependencies {
        registry.Register(dep)
    }
}
```

**3-Tier Discovery:**
1. **Manifest tier**: plugin.json에 명시된 entry points
2. **Capability tier**: 런타임 capability query (init() hook에서 선언)
3. **Schema tier**: JSON schema from manifest 자동 로드

**4 primitives:**
- Load/Unload (lifecycle)
- Query (capability discovery)
- Call (invocation)
- Monitor (health check)

**Trade-off:** 원자성 보장 위해 clear-all이 필요 (≈50ms 다운타임)

### ADR-022: Per-Triple Lock Permission (PERMISSION-001)

**Decision:** 권한은 (user, tool, resource) 3-tuple별로 독립적 잠금

**Architecture:**
```
Triple = (UserID, ToolName, ResourcePath)
Example: ("alice@example.com", "exec-bash", "/etc/passwd")

Lock states:
  - None (미승인)
  - Declared (메니페스트에서 선언)
  - Approved (사용자가 수동 승인)
  - FirstCall (첫 호출 시에만 확인)
```

**First-Call Confirm:**
- 새로운 (user, tool, resource) triple은 첫 호출 시 사용자 확인
- 확인 후: "Always Allow" / "This Time Only" / "Deny" 선택
- "Always Allow" 선택 시 메모리에만 저장 (미지속)
- 세션 종료 시 "Always Allow" 사라짐

**Declared vs Runtime:**
- Declared: 코드에 명시 (read-only, network URL 등)
- Runtime: 동적 결정 (unknown resource path)

**Trade-off:** per-triple 저장소는 큐리 많음 (수십만 rows), SQLite FTS 인덱싱 필수

### ADR-023: Brand-Lint CI Gate (BRAND-RENAME-001)

**Decision:** 모든 PR은 merge 전에 `scripts/check-brand.sh` 통과 필수

**Rules:**
- 모든 user-facing prose에 "MINK" 사용 (대문자, 마침표 후)
- 코드 식별자는 `mink` (소문자, 영어만)
- 제외: `.moai/specs/SPEC-GOOSE-BRAND-RENAME-001/` (원본 보존)
- Fallback: SPEC 파일의 HISTORY 섹션은 lint 제외

**CI Integration:**
- `.github/workflows/brand-lint.yml` 신규 추가
- `scripts/check-brand.sh` 로컬 + CI에서 실행
- Exit code 0 = pass, 1 = fail (PR 블로킹)

**Trade-off:** 엄격한 규칙은 maintainer의 수동 요청 증가 가능

---

## 9. LLM Provider 생태계 (SPEC-GOOSE-ADAPTER-001, SPEC-GOOSE-ADAPTER-002)

MINK는 `internal/llm/provider/` 하위에 복수 LLM provider 어댑터를 통합한다. 모두 공통 `Provider` 인터페이스를 구현하며 `ProviderRegistry`에 등록된다. SPEC-001 (6 provider) + SPEC-002 (9 provider) 병합으로 총 **15 provider adapter-ready**.

### 9.1 현재 지원 (15 provider adapter-ready)

#### SPEC-001 (native / OAuth / SDK 기반 6종)

| Provider | 지원 모델 | 특징 | 상태 |
|----------|---------|------|------|
| **Anthropic** | Claude Sonnet 4.6, Opus 4.7 | OAuth PKCE, Adaptive Thinking (effort), budget_tokens LEGACY | ✅ GREEN |
| **OpenAI** | GPT-4o, GPT-4 Turbo, GPT-3.5-turbo, o1-preview | base_url 교체 가능, tool_calls aggregation, ExtraHeaders/ExtraRequestFields | ✅ GREEN |
| **xAI Grok** | Grok-2, Grok-3 (vision) | OpenAI-compatible 팩토리 | ✅ GREEN |
| **DeepSeek** | DeepSeek-Chat, DeepSeek-Reasoner (r1) | OpenAI-compatible, vision=false | ✅ GREEN |
| **Google Gemini** | Gemini 2.0 Flash, Pro | `google.golang.org/genai` SDK + fake client 추상화 | ✅ GREEN |
| **Ollama** | llama2, mistral, neural-chat 등 | 로컬 모델, /api/chat JSON-L 스트리밍, 무인증 | ✅ GREEN |

#### SPEC-002 (OpenAI-compat 기반 9종)

| Provider | 지원 모델 | 특징 | 상태 |
|----------|---------|------|------|
| **Z.ai GLM** | glm-5, glm-4.7, glm-4.6, glm-4.5, glm-4.5-air | thinking mode (4 모델) + `api.z.ai` 공식 이관, ExtraRequestFields 활용 | ✅ GREEN |
| **Groq** | Llama 3.3/4, DeepSeek R1 Distill, Mixtral 8x7B (free) | LPU 315 TPS, 무료 tier (30 RPM / 14.4K RPD) | ✅ GREEN |
| **OpenRouter** | 300+ 모델 gateway (GPT-OSS, Qwen3-Coder, Nemotron 등) | 29 free model, `HTTP-Referer` / `X-Title` 랭킹 헤더 주입 | ✅ GREEN |
| **Together AI** | Llama 3.3 70B Turbo, Qwen2.5, Mixtral 8x22B 등 173 모델 | Fine-tuning 지원, 55/101 공유모델 Fireworks보다 저렴 | ✅ GREEN |
| **Fireworks AI** | Llama, DeepSeek R1, Qwen3-Coder 480B 등 209 모델 | 145 TPS, 50% cached+batch 할인 | ✅ GREEN |
| **Cerebras** | Llama 3.3 70B, Llama 3.1 8B | Wafer-Scale Engine 1,000+ TPS (속도 1위) | ✅ GREEN |
| **Mistral AI** | Mistral Nemo ($0.02/M 최저가), Small, Medium, Codestral 등 42 모델 | 오픈소스 친화, 자체 API | ✅ GREEN |
| **Qwen (DashScope)** | qwen3-max, qwen3.6-max-preview, qwen3-coder-plus, qwen3-vl | Region: intl/cn/sg/hk (env + option 3단계), 1T MoE 2026-04 | ✅ GREEN |
| **Kimi (Moonshot)** | kimi-k2.6 (1T MoE, 262K context), k2.5, moonshot-v1-128k | Region: intl/cn (env + option 3단계), OpenAI+Anthropic 양쪽 호환 | ✅ GREEN |

### 9.2 계획 중 (후속 SPEC 후보)

- **Perplexity Sonar** — web search 내장 (tool cost $0.005/search) 별도 SPEC 예정
- **MiniMax / Nous / Naver HyperCLOVA X / KT Mi:dm** — 수요 검증 후 등록

### 9.3 아키텍처 개요

**공통 인터페이스** (`internal/llm/provider/provider.go`):

```go
type Provider interface {
    Complete(ctx context.Context, req *LLMCallReq) (*CompletionResponse, error)
    Stream(ctx context.Context, req *LLMCallReq) (*StreamReader, error)
    Capabilities() ProviderCapabilities
}

type ProviderCapabilities struct {
    Vision           bool
    FunctionCalling  bool
    Streaming        bool
    VisionModels     []string
    MaxContextWindow int
}
```

**Registry** (`internal/llm/provider/registry.go`):
- 런타임 provider 라우팅
- 모델명 → 프로바이더 매핑 (예: "claude-opus-4-7" → Anthropic)
- Fallback model chain (5xx/network error 시 순차 재시도)

### 9.4 공통 기능

| 기능 | REQ | 구현 |
|-----|-----|------|
| **Streaming** | REQ-ADAPTER-013 | 60s heartbeat timeout watchdog |
| **429 회전** | REQ-ADAPTER-008 | credential.MarkExhaustedAndRotate() |
| **Fallback chain** | REQ-ADAPTER-009 | TryWithFallback() helper |
| **Vision pre-check** | REQ-ADAPTER-017 | ErrCapabilityUnsupported 거절 |
| **PII 로깅 방지** | REQ-ADAPTER-014 | 민감 정보 마스킹 |
| **ExtraHeaders** | REQ-ADAPTER-016 | provider-specific 헤더 주입 |
| **ExtraRequestFields** | REQ-ADAPTER-016 | body merge (provider 파라미터 pass-through) |

### 9.5 각 어댑터 상세

**Anthropic 어댑터** (`internal/llm/provider/anthropic/`):
- OAuth PKCE refresh (Claude API credentials → session token)
- Thinking mode 듀얼 경로:
  - Claude 4.6 이하: fixed `budget_tokens` (LEGACY)
  - Opus 4.7 Adaptive: `effort: xhigh` (동적 할당, REQ-OPUS-ADAPTIVE)
- SSE streaming with `stream: true`
- Tool schema 변환 (MoAI → Anthropic format)
- MaxTokens clamping (모델별 최대값 준수)

**OpenAI 어댑터** (`internal/llm/provider/openai/`):
- 일반 OpenAI-compatible 팩토리 (기본값: api.openai.com)
- GPT-4o, GPT-4 turbo, GPT-3.5-turbo, o1-preview 지원
- base_url 교체 가능 (Azure, Fireworks, Anyscale 등)
- tool_calls aggregation (중복 호출 병합)
- 60s heartbeat timeout

**xAI Grok 어댑터** (`internal/llm/provider/xai/`):
- OpenAI 팩토리 래핑
- BaseURL: api.x.ai 자동 설정
- Grok-2, Grok-3 vision 지원

**DeepSeek 어댑터** (`internal/llm/provider/deepseek/`):
- OpenAI 팩토리 래핑
- BaseURL: api.deepseek.com
- DeepSeek-Chat, DeepSeek-Reasoner (r1) 지원
- Vision=false capability 명시 (REQ-ADAPTER-017)

**Google Gemini 어댑터** (`internal/llm/provider/google/`):
- google.golang.org/genai SDK 기반
- fake client 추상화 (테스트용 mock 지원)
- Gemini 2.0 Flash, Pro 지원
- 60s heartbeat timeout

**Ollama 어댑터** (`internal/llm/provider/ollama/`):
- 로컬 모델 지원
- /api/chat JSON-L 스트리밍
- llama2, mistral, neural-chat, openchat 등
- 무인증 (localhost 기본값)
- 모델 동적 발견 (LIST /api/tags)

### 9.6 온디바이스 추론

**Rust mink-ml** (ML/LoRA 훈련):
- Candle (Huggingface) → 고속 추론
- Burn (Rust ML) → GPU 지원
- tch-rs (PyTorch binding)
- ort (ONNX Runtime)
- safetensors (안전한 tensor 직렬화)
- 로컬 GGUF 모델 지원 (Ollama 연동)

### 9.7 글로벌 프로바이더 우선순위 (SPEC-002 반영)

| 우선순위 | 프로바이더 | 상태 |
|---------|----------|------|
| **P0** | Anthropic, OpenAI, Google | ✅ ACTIVE |
| **P1** | Ollama, xAI, DeepSeek | ✅ ACTIVE |
| **P2** | Mistral, Groq, OpenRouter, Together, Fireworks, Cerebras | ✅ ACTIVE (SPEC-002) |
| **P3** | Z.ai GLM, Qwen, Kimi | ✅ ACTIVE (SPEC-002) |
| **P4** | Naver HyperCLOVA X, KT Mi:dm, Perplexity, MiniMax | 🔲 TODO (후속 SPEC) |

---

**온디바이스 추론**: Rust **mink-ml** (candle, ort)

---

## 10. 에이전트 하네스 & 평가 (MoAI 계승)

Go 기반 harness (MoAI 계승):
- minimal / standard / thorough 3-level

벤치마크:
- MASEval (multi-agent)
- HAL (ICLR 2026)
- GOOSE-Bench (개인화, 자체)

**Rust ML 레이어 별도 벤치마크**:
- Candle vs PyTorch (Huggingface 기준)
- hnsw-rs vs hnswlib
- snow vs noise-rust

---

## 11. 개발 환경

### 11.1 필수 도구

| 도구 | 버전 | 용도 |
|-----|------|------|
| **Rust** | 1.80+ | Critical 레이어 |
| **Go** | 1.26+ | Orchestration |
| **Node** | 22+ LTS | TypeScript |
| **protoc + buf** | 최신 | Protobuf |
| **cbindgen** | 최신 | Rust → C 헤더 |
| **cargo-make** | 최신 | Rust 빌드 |
| **Turborepo** | 최신 | TS 모노레포 |
| **Docker** | 24+ | 통합 테스트 |
| **Ollama** | 최신 | 로컬 LLM |

### 11.2 환경 변수

```bash
# 필수
MINK_HOME=~/.mink
MINK_LOCALE=en|ko|ja|zh

# Rust build
CARGO_BUILD_JOBS=8
RUSTFLAGS="-C target-cpu=native"  # SIMD 최적화

# Go build
GOFLAGS=-mod=vendor
CGO_ENABLED=1  # Rust FFI용

# TypeScript
TURBO_TOKEN=...

# LLM API (기존)
OPENAI_API_KEY=...
ANTHROPIC_API_KEY=...
OLLAMA_HOST=http://localhost:11434

# Learning Engine
MINK_LEARNING_ENABLED=true
MINK_LORA_TRAINING=true  # Rust mink-ml
MINK_FEDERATED_LEARNING=false

# Privacy
MINK_ENCRYPTION=true  # Rust mink-crypto
MINK_RELAY_URL=wss://relay.mink.dev
```

---

## 12. 테스트 전략 (3-언어)

### 12.1 Rust 테스트

- `cargo test` (단위)
- `cargo test --test integration` (통합)
- `cargo bench` (criterion)
- `cargo fuzz` (퍼징)
- 언세이프 코드 = **0 허용** (Agentor 패턴)

### 12.2 Go 테스트

- `go test -race -cover`
- testify, mockery
- gofuzz
- 테스트 커버리지 85%+

### 12.3 TypeScript 테스트

- vitest, ink-testing-library
- Playwright (E2E)
- Detox (React Native)

### 12.4 통합 테스트 (3-언어)

- Rust ↔ Go FFI 테스트
- gRPC 프로토 버전 호환
- E2EE 왕복 (Rust Relay ↔ Go Client)

---

## 13. 배포 전략 (3-언어)

### 13.1 배포 대상

| 대상 | 포함 언어 | 방법 |
|-----|---------|------|
| CLI binary | Go + Rust FFI | GitHub Releases |
| Desktop | Rust (Tauri) + TS | Tauri 서명, auto-update |
| iOS/Android | TS + Rust (ExecuTorch) | App Store, Play Store |
| Web | TS | Vercel |
| npm | TS | `@gooseagent/*` |
| Cargo | Rust | crates.io (SDK) |
| Docker | Go + Rust | Multi-arch |
| Go module | Go | pkg.go.dev |

### 13.2 Docker 3-언어 빌드

```dockerfile
# Stage 1: Rust 빌드
FROM rust:1.80-alpine AS rust-builder
WORKDIR /app/crates
COPY crates ./
RUN cargo build --release

# Stage 2: Go 빌드 (Rust 링크)
FROM golang:1.26-alpine AS go-builder
WORKDIR /app
COPY --from=rust-builder /app/crates/target/release/libmink_ml.a /usr/lib/
COPY . .
RUN CGO_ENABLED=1 go build -o minkd ./cmd/minkd/

# Stage 3: TS 빌드 (필요시)
FROM node:22-alpine AS ts-builder
WORKDIR /app
COPY packages ./packages
RUN cd packages && bun install && turbo build

# Stage 4: 최종 이미지
FROM alpine:3.20
RUN apk --no-cache add ca-certificates
COPY --from=go-builder /app/minkd /usr/local/bin/
EXPOSE 50051
ENTRYPOINT ["minkd"]
```

---

## 14. 연구 참조 (2026 최신)

### 14.1 하이브리드 아키텍처 참조
- **Cloudflare Pingora** (Rust 프록시)
- **AWS Firecracker** (Rust VM)
- **Microsoft Wassette** (Rust + WASM + MCP, 2026.04)
- **Mullvad GotaTun** (Go → Rust WireGuard, 2026)
- **Agentor** (13 Rust 크레이트, WASM AI 에이전트)

### 14.2 Go AI 생태계
- **Eino** (CloudWeGo/ByteDance)
- **Firebase Genkit** (Google)
- **ADK-Go** (Google)
- **LangChainGo**

### 14.3 Rust AI/ML 생태계
- **Candle** (Huggingface)
- **Burn** (Rust ML)
- **Agentor** (13-crate framework)
- **tch-rs** (PyTorch binding)

### 14.4 학술 논문
- arXiv:2507.21046 (Self-Evolving Agents)
- arXiv:2508.07407 (Comprehensive Survey)
- EWC, LwF, DoRA
- Noise Protocol Framework

### 14.5 규제 & 표준
- OWASP Top 10 Agentic Apps 2026.12
- MCP (Linux Foundation)
- A2A v0.3 (Linux Foundation)
- EU AI Act 2026.08
- ERC-8004 Trustless Agents

---

## 15. 레이어별 언어 선택 체크리스트

### 15.1 Rust 선택 기준 (YES = Rust)

- [ ] 메모리 안전이 보안 크리티컬인가?
- [ ] 레이턴시가 p99 < 10ms 필요한가?
- [ ] SIMD/GPU 최적화 필요한가?
- [ ] 암호화 primitive를 직접 다루는가?
- [ ] OS 네이티브 API 필요한가?
- [ ] WASM 런타임인가?

### 15.2 Go 선택 기준 (YES = Go)

- [ ] 동시 IO 핸들링인가?
- [ ] 에이전트 오케스트레이션인가?
- [ ] HTTP/gRPC 서비스인가?
- [ ] MoAI-ADK-Go 재사용 가능한가?
- [ ] 개발 속도가 우선인가?
- [ ] GC 허용 가능한가?

### 15.3 TypeScript 선택 기준 (YES = TS)

- [ ] 사용자 UI인가?
- [ ] 브라우저 환경인가?
- [ ] Ink/React 생태계 필요한가?
- [ ] 빠른 iteration이 우선인가?

---

## 16. 결론

**v4.1 최종 기술 스택**:

```
🦀 Rust (20% - Critical)
  ├─ ML/LoRA 훈련 (mink-ml)
  ├─ WASM 샌드박스 (mink-wasm)
  ├─ E2EE 암호화 (mink-crypto)
  ├─ Desktop OS API (mink-desktop)
  └─ Vector 연산 (mink-vector)

🐹 Go (70% - Orchestration)
  ├─ minkd daemon (MoAI-ADK-Go 계승)
  ├─ Agent Runtime
  ├─ Learning Engine Core
  ├─ LLM Router
  ├─ Tools + Memory + Transport
  └─ Gateway + MCP

📘 TypeScript (10% - UI)
  ├─ mink-cli (Ink)
  ├─ mink-desktop (Tauri)
  ├─ mink-mobile (React Native)
  └─ mink-web (Next.js 15)
```

**원칙**:
> "Go는 80% 성능을 20% 노력으로. Rust는 20% 성능을 80% 노력으로. 
> 각자 최적의 자리에."

**결정 근거**:
- 2026 업계 표준 (Cloudflare, AWS, Mullvad, Microsoft)
- MoAI-ADK-Go 38,700줄 재사용
- 보안 크리티컬 = Rust 확정
- ML 성능 = Rust 10x
- Go 개발 속도 = 유지

---

Version: **4.1.0 POLYGLOT HYBRID** (v4.0 Go-only → v4.1 Rust+Go+TS)
Language: **Rust 1.80+ + Go 1.26+ + TypeScript 5.x**
Created: 2026-04-21
Updated: 2026-04-21
License: Apache-2.0
Base: MoAI-ADK-Go (38,700 LOC) + Rust security layer + TS UI
Reference: Claude Code + Hermes Agent + MoAI-ADK + 2026 Industry patterns

> **"Rust for safety. Go for velocity. TypeScript for UX. All together for GOOSE."**

Sources:
- [Rust vs Go 2026 Benchmarks](https://tech-insider.org/rust-vs-go-2026/)
- [Wasmtime Security (Bytecode Alliance)](https://docs.wasmtime.dev/security.html)
- [Microsoft Wassette for AI Agents](https://opensource.microsoft.com/blog/2025/08/06/introducing-wassette-webassembly-based-tools-for-ai-agents/)
- [Mullvad GotaTun Go→Rust](https://www.techradar.com/vpn/vpn-services/mullvad-vpn-boosts-wireguard-speeds-and-stability-with-new-rust-based-engine)
- [Go AI Concurrency](https://dasroot.net/posts/2026/02/high-concurrency-ai-apis-goroutines/)
- [Polyglot Microservices 2026](https://crazyimagine.com/blog/high-performance-hybrid-architectures-rust-vs-go-in-2026/)
- [Rust Go FFI CGO](https://github.com/mediremi/rust-plus-golang)
- [Agentor - Rust AI Framework](https://www.xcapit.com/en/blog/from-openclaw-to-agentor-building-secure-ai-agents-in-rust)
