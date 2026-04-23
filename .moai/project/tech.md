# GOOSE-AGENT - Technology Document v4.1 POLYGLOT HYBRID

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

### 0.3 GOOSE의 선택 근거

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
│  goose-cli (Ink) │ goose-desktop (Tauri) │                   │
│  goose-mobile (RN) │ goose-web (Next.js)                     │
├──────────────────────────────────────────────────────────────┤
│  Bridge: gRPC (.proto contracts, cross-language)             │
├──────────────────────────────────────────────────────────────┤
│  🐹 Go Layer (Orchestration Majority - 70% LOC)              │
│  goosed daemon │ Agent Runtime │ Learning Coord              │
│  LLM Router │ Tools │ Memory │ Transport │ Gateway           │
├──────────────────────────────────────────────────────────────┤
│  Bridge: gRPC + CGO (hot paths only)                         │
├──────────────────────────────────────────────────────────────┤
│  🦀 Rust Layer (Performance & Security Critical - 20% LOC)   │
│  goose-ml (LoRA 훈련, Vector 연산)                            │
│  goose-wasm (Extism/Wasmtime 샌드박스)                        │
│  goose-crypto (E2EE Relay, Noise Protocol)                   │
│  goose-desktop (OS Accessibility API)                        │
│  goose-graph (Identity Graph 최적화)                          │
├──────────────────────────────────────────────────────────────┤
│  Infrastructure                                               │
│  SQLite/FTS5 │ Qdrant │ Graphiti │ Ollama │ WASI            │
└──────────────────────────────────────────────────────────────┘
```

### 1.2 레이어별 언어 매핑 (최종 결정)

| # | 레이어 | 언어 | 근거 | 참조 |
|---|-------|-----|------|------|
| 1 | **Daemon 오케스트레이터** (goosed) | 🐹 **Go** | MoAI-ADK-Go 계승, goroutines | MoAI-ADK |
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

**goose-ml** (ML/LoRA 훈련):
| 의존성 | 용도 |
|-------|------|
| **candle-core** | Huggingface의 Rust ML 프레임워크 |
| **burn** | Pure Rust ML 프레임워크 (GPU 지원) |
| **tch-rs** | PyTorch Rust 바인딩 |
| **ort** | ONNX Runtime Rust |
| **safetensors** | 안전한 tensor 직렬화 |
| **ndarray** | NumPy-like 배열 |
| **rayon** | 데이터 병렬 처리 |

**goose-wasm** (WASM 샌드박스):
| 의존성 | 용도 |
|-------|------|
| **wasmtime** | Microsoft Wassette 기반 |
| **extism** | 다국어 플러그인 SDK |
| **wit-bindgen** | Component Model |
| **cap-std** | Capability-based std |

**goose-crypto** (E2EE & Relay):
| 의존성 | 용도 |
|-------|------|
| **snow** | Noise Protocol (WireGuard 동일) |
| **boring** | BoringSSL 래퍼 (Cloudflare) |
| **ring** | 암호 primitives |
| **x25519-dalek** | X25519 KEM |
| **chacha20poly1305** | AEAD |
| **argon2** | 패스워드 해싱 |

**goose-desktop** (OS Accessibility):
| 의존성 | 용도 |
|-------|------|
| **accessibility-ng** | macOS AXUIElement |
| **screencapturekit-rs** | macOS 스크린 |
| **enigo** | 크로스 플랫폼 입력 |
| **core-graphics** | macOS CG |
| **windows-rs** | Windows UIA |
| **atspi** | Linux AT-SPI2 |

**goose-vector** (Vector 연산):
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
    "crates/goose-ml",
    "crates/goose-wasm",
    "crates/goose-crypto",
    "crates/goose-desktop",
    "crates/goose-vector",
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
| `crates/goose-ml/goose_ml.h` | Rust → C 헤더 |

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

### 4.1 goose-cli (Terminal)
- ink 6.x (ESM), react 19.x, @inkjs/ui 3.x, commander 12.x
- @connectrpc/connect 2.x (gRPC 클라이언트)

### 4.2 goose-desktop (Tauri v2)
- tauri 2.x (Rust backend 자체적)
- react 19.x, zustand 5.x, tailwindcss 4.x
- shadcn/ui, framer-motion 11.x

### 4.3 goose-mobile (React Native)
- react-native 0.76+ (New Architecture)
- @picovoice/porcupine-react-native 3.x
- whisper.rn 0.4+
- react-native-executorch 0.2+ (LoRA)
- react-native-stripe

### 4.4 goose-web (Next.js 15)
- next 15.x, react 19.x
- @connectrpc/connect-web 2.x

---

## 5. 빌드 시스템 (3-언어)

### 5.1 통합 빌드 파이프라인

```bash
# 1. Rust 크레이트 빌드 (goose-ml, goose-wasm 등)
cd crates
cargo build --release
cbindgen --crate goose-ml --output goose_ml.h  # C 헤더 생성

# 2. Go 바이너리 빌드 (Rust 의존성 link)
cd ..
go build -o bin/goosed ./cmd/goosed/
go build -o bin/goose ./cmd/goose/

# 3. TypeScript 빌드 (Turborepo)
cd packages
turbo build

# 4. Tauri 데스크톱 (Rust + TS)
cd packages/goose-desktop
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

### 6.1 E2EE (Rust goose-crypto)

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

### 6.2 WASM Sandbox (Rust goose-wasm)

**Wasmtime 기반 (Microsoft Wassette 패턴)**:
- 메모리 격리
- 콜스택 접근 불가
- 메모리 제로화 (누수 방지)
- Capability-based 권한

**Extism 다국어 플러그인**:
- Rust, Go, C, AssemblyScript, Zig 지원
- OCI 레지스트리 + OSV 스캔

### 6.3 OS Accessibility (Rust goose-desktop)

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

**Update:** LoRA 훈련 = **Rust goose-ml**. On-device QLoRA 성능 극대화.

### ADR-013: Privacy-preserving Stack

**Update:** E2EE Relay = **Rust goose-crypto** (Mullvad GotaTun 패턴).

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

---

## 9. LLM 프로바이더 지원 (유지)

글로벌 프로바이더 (Go LLM Router 담당):
- Anthropic Claude (P0)
- OpenAI GPT (P0)
- Google Gemini (P0)
- Ollama local (P1)
- xAI, OpenRouter, DeepSeek, Mistral (P1-P2)
- Naver HyperCLOVA X, KT Mi:dm (P3)

**온디바이스 추론**: Rust **goose-ml** (candle, ort)

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
GOOSE_HOME=~/.goose
GOOSE_LOCALE=en|ko|ja|zh

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
GOOSE_LEARNING_ENABLED=true
GOOSE_LORA_TRAINING=true  # Rust goose-ml
GOOSE_FEDERATED_LEARNING=false

# Privacy
GOOSE_ENCRYPTION=true  # Rust goose-crypto
GOOSE_RELAY_URL=wss://relay.gooseagent.org
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
COPY --from=rust-builder /app/crates/target/release/libgoose_ml.a /usr/lib/
COPY . .
RUN CGO_ENABLED=1 go build -o goosed ./cmd/goosed/

# Stage 3: TS 빌드 (필요시)
FROM node:22-alpine AS ts-builder
WORKDIR /app
COPY packages ./packages
RUN cd packages && bun install && turbo build

# Stage 4: 최종 이미지
FROM alpine:3.20
RUN apk --no-cache add ca-certificates
COPY --from=go-builder /app/goosed /usr/local/bin/
EXPOSE 50051
ENTRYPOINT ["goosed"]
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
  ├─ ML/LoRA 훈련 (goose-ml)
  ├─ WASM 샌드박스 (goose-wasm)
  ├─ E2EE 암호화 (goose-crypto)
  ├─ Desktop OS API (goose-desktop)
  └─ Vector 연산 (goose-vector)

🐹 Go (70% - Orchestration)
  ├─ goosed daemon (MoAI-ADK-Go 계승)
  ├─ Agent Runtime
  ├─ Learning Engine Core
  ├─ LLM Router
  ├─ Tools + Memory + Transport
  └─ Gateway + MCP

📘 TypeScript (10% - UI)
  ├─ goose-cli (Ink)
  ├─ goose-desktop (Tauri)
  ├─ goose-mobile (React Native)
  └─ goose-web (Next.js 15)
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
License: MIT
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
