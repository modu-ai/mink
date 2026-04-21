# GENIE-AGENT - Migration Guide & Pivot Journey v4.0

> **문서 목적:** 5번의 피벗을 거쳐 GENIE에 도착한 여정을 기록. 새 팀원, 기여자, 이해관계자를 위한 프로젝트 스토리.

---

## 0. 문서 개요

이 문서는 **솔직한 자기 반성 문서**이다.

프로젝트가 거쳐온 5번의 피벗을 숨기지 않고 기록한다:
- v0.2 CLI Tool (2026-04-09)
- v1.0 Oz World (2026-04-10)
- v2.0 Genie Global (2026-04-10)
- v3.0 Genie Korea (2026-04-21)
- **v4.0 GENIE Global** (2026-04-21 ~)

각 피벗의 **이유와 학습**을 정리하여, 앞으로 같은 실수 반복을 방지한다.

---

## 1. 전체 피벗 타임라인

```
2026-04-09 ━━━ v0.2 CLI Tool
                ├─ 개발자용 단순 CLI
                └─ Rust + TS

2026-04-10 ━━━ v1.0 Oz World
                ├─ Summer Wars 메타버스
                └─ 거대 에이전트 생태계

2026-04-10 ━━━ v2.0 Genie Global
                ├─ 아라비안나이트 지니
                └─ PC + Mobile 듀얼 액세스

2026-04-21 ━━━ v3.0 Genie Korea
                ├─ KT 기가지니 이식
                └─ 한국 매각 타깃 (500-800억원)

2026-04-21 ━━━ v4.0 GENIE Global ⭐
                ├─ 자기진화 개인화 AI
                ├─ Go + TypeScript
                └─ 글로벌 오픈소스 (MIT)

2026-04-21 ━━━ v5.0 GENIE Eternal 🪔
                ├─ 지니 메타포 완전 재구성
                ├─ 램프/소원/영원한 주종 계약
                └─ 마법의 은유 체계
```

**12일 동안 5번의 피벗**. 이것은 정상이 아니다.
하지만 각 피벗에서 배운 것들이 v4.0에 녹아있다.

**v4.0 → v5.0 진화 (2026-04-21)**: 
기술적 기반은 동일하되, 브랜드 메타포를 거위에서 지니로 재구성.
"자기진화하는 AI"라는 핵심은 유지하면서, 더 강력한 "영원한 주종 계약" 은유로 승격.

---

## 2. 각 버전 상세 분석

### 2.1 v0.2: CLI Tool (Starting Point)

**날짜**: 2026-04-09
**언어**: Rust + TypeScript
**컨셉**: Claude Code / Hermes 영감 CLI

**특징**:
- 개발자용 단순 CLI
- 터미널 UI (Ink/Ratatui)
- Tool + Agent 기본 구조

**Why this direction?**
- Claude Code, Hermes 분석 후 비슷한 것 만들자는 발상
- 개발자 시장이 가장 접근 쉬움
- "Proof of concept" 단계

**교훈 (Learnings)**:
- "Claude Code 복제"로는 차별화 어려움
- CLI는 개발자만 쓴다 → 시장 제한
- 독자적 비전 부재

---

### 2.2 v1.0: Oz World (2026-04-10)

**날짜**: 2026-04-10
**언어**: Rust + TypeScript
**컨셉**: Summer Wars 가상 세계

**특징**:
- 거대 메타버스 에이전트 생태계
- 3-tier 에이전트 계층 (Super/Org/Worker)
- 마켓플레이스 경제
- 가상 세계 은유

**Why this direction?**
- 더 큰 비전 필요
- Summer Wars 영감 (일본 애니)
- "에이전트가 살아가는 세계"

**교훈 (Learnings)**:
- 너무 거창 (메타버스)
- 개인 사용자에게 추상적
- "실제로 뭐 할 수 있는지" 불명확
- 투자자/사용자 이해 어려움

---

### 2.3 v2.0: Genie Global (2026-04-10, 같은 날)

**날짜**: 2026-04-10
**언어**: Rust + TypeScript
**컨셉**: 아라비안 나이트 지니

**특징**:
- 개인 AI 비서 메타포
- 듀얼 액세스 (PC + Mobile)
- 토큰 경제 도입
- 글로벌 타깃

**Why this direction?**
- "지니" 메타포 = 개인 비서로 친숙
- 더 구체적이고 이해 쉬움
- 모바일 + PC 현실적 UX

**교훈 (Learnings)**:
- "Genie" 이름 중복 (KT 기가지니, Warp Genie 등)
- 글로벌 + 개인화 균형 어려움
- 초기 MVP 어떻게 잡을지 불명확

---

### 2.4 v3.0: Genie Korea (2026-04-21)

**날짜**: 2026-04-21
**언어**: Rust + TypeScript
**컨셉**: KT 기가지니 이식 + 매각

**특징**:
- 한국 전용 시장 집중
- KT Mi:dm LLM 우선 통합
- 기가지니 스피커 이식 레이어
- 목표: **KT에 500-800억원 매각**
- Phase 1-4 상세 로드맵 (24개월)

**Why this direction?**
- "구체적이고 실행 가능한 목표"
- "KT 매각"이라는 명확한 Exit
- 한국 시장 + 기존 자산 활용

**교훈 (Learnings)** ⚠️ (가장 큰 교훈):
- **한국 M&A 비율 2%** (미국 80% vs)
- KT 의사결정 느림 (12개월+)
- 한국 시장 크기 작음 (글로벌 대비 1/50)
- 오픈소스 비전과 모순
- Mi:dm 팀과 충돌 가능성

**왜 v3.0에서 v4.0으로 피벗했나?**
- KT 매각 시나리오의 비현실성 인식
- **MoAI-ADK-Go 발견**: SPEC-REFLECT-001 이미 구현됨
- 자기진화 = 글로벌 차별화 가능 영역
- 오픈소스 = 지속 가능성

---

### 2.5 v4.0: GENIE Global (2026-04-21 ~, **현재**)

**날짜**: 2026-04-21 ~
**언어**: **Go + TypeScript** (Rust → Go)
**컨셉**: **자기진화 개인화 AI**

**특징**:
- 3개 소스 프로젝트 통합 (Claude Code + Hermes + MoAI-ADK)
- 자기진화 학습 엔진 (SPEC-REFLECT-001 계승)
- 100% 사용자 개인화 (User LoRA)
- 글로벌 오픈소스 (MIT)
- 거위 메타포 (평생 동반자)

**Why this is DIFFERENT (실제로 최종 방향):**

1. **명확한 기술 차별점**:
   - 자기진화 = 모든 경쟁사 없음
   - MoAI-ADK에서 이미 검증된 엔진

2. **현실적 실행 가능성**:
   - Go = MoAI-ADK 38,700줄 재사용
   - 오픈소스 = 리소스 낮음
   - 커뮤니티 = 지속 가능

3. **장기 브랜드**:
   - "GENIE" = 사용자명 goos 연결 = 개인 애착
   - 거위 = 평생 동반자 은유와 일치
   - 영구 독립 브랜드 (인수 X)

4. **글로벌 시장**:
   - 한국 제한 X
   - 오픈소스 커뮤니티 글로벌
   - 다국어 지원

---

### 2.6 v5.0: GENIE Eternal (2026-04-21, **현재 진화 단계**)

**날짜**: 2026-04-21 ~
**언어**: Go + TypeScript (v4.0과 동일)
**컨셉**: **지니 메타포 완전 재구성 (거위 → 지니)**

**특징**:
- 기술 기반: v4.0 100% 계승 (변경 없음)
- 브랜드 메타포: 거위 → 지니 (마법, 영원성)
- 은유 체계: 
  - 🪔 봉인된 램프 (설치 후)
  - 💨 소환 (첫 대화)
  - 🔮 주인의 마음 읽기 (월 1-3)
  - 💎 영원한 동반자 (년 1+)
- 커뮤니티 용어:
  - Gaggle → Genie Assembly (지니 회합)
  - Flight School → Lamp School (램프학교)
  - Feather → Wish Spark (소원 불씨)
  - Migration → Wish Journey (소원 여정)

**Why this evolution (기술 아님, 순수 브랜딩)**:
1. **더 강한 비유**:
   - 거위 = 자연의 관찰자 (좋지만 보편적)
   - 지니 = 초자연적 봉사자 (더 신비, 더 강함)

2. **영원성 강조**:
   - 거위 "평생 짝" → 지니 "영원한 주종 계약"
   - 법적/마법적 구속력 강화

3. **글로벌 인식**:
   - 거위는 문화적으로 특정 (유럽/미국 편향)
   - 지니는 보편적 (중동 출신이지만 홍콩, 디즈니로 글로벌화)

4. **KT 기가지니와의 명확한 차별화**:
   - 거위는 혼동 가능성 (다른 동물)
   - 지니는 개념적으로 명확 (초자연/마법 vs 스피커)

**기술 변경 사항**: **NONE** (순수 metaphor rebranding)

**문서 변경 범위**:
- README.md (영어)
- branding.md (한국어) — §2 거위 은유체계 → 지니 은유체계
- product.md (한국어) — 버전 테이블에 v5.0 추가
- migration.md (한국어) — 이 섹션 (현재 작성 중)
- adaptation.md, ecosystem.md, learning-engine.md — 메타포 참조만 수정
- ROADMAP.md, IMPLEMENTATION-ORDER.md — 메타포 언급 최소화

---

## 3. 6번의 피벗 속에서 유지된 것 (Invariants)

피벗이 많았지만, 몇 가지는 절대 안 바뀌었다:

### 3.1 기술 아키텍처 패턴

- **에이전트 + 도구 모델**: v0.2 ~ v5.0 동일
- **다중 LLM 지원**: 모든 버전
- **WASM 플러그인 샌드박스**: 모든 버전
- **크로스 플랫폼**: Desktop + Mobile + Web

### 3.2 핵심 영감 프로젝트

- **Claude Code** (TypeScript): UI 패턴, Tool 인터페이스
- **Hermes Agent** (Python): 자기개선, 메모리 프로바이더
- **MoAI-ADK** (Go): 자기진화, @MX tags, SPEC workflow

v0.2부터 이 세 프로젝트 분석이 기반.

### 3.3 개발 철학

- **오픈소스 우선**: 모든 버전에서 (v3.0 잠시 상업적 기울었음)
- **프라이버시 중요**: 모든 버전
- **TRUST 5 품질**: MoAI-ADK 계승
- **한국어 1급 지원**: 창립자 모국어

### 3.4 UX 원칙

- **대화형 AI**: CLI + GUI
- **사용자 승인**: 위험 작업
- **투명성**: 무엇을 하는지 알림

---

## 4. 피벗마다 바뀐 것 (Variants)

### 4.1 언어 변화

| 버전 | 코어 언어 | 클라이언트 | 이유 |
|-----|---------|----------|------|
| v0.2 | Rust | TS | 성능 우선 |
| v1.0 | Rust | TS | 유지 |
| v2.0 | Rust | TS | 유지 |
| v3.0 | Rust | TS (Korean adapter) | 유지 |
| **v4.0** | **Go** | **TS** | **MoAI-ADK-Go 계승** ⭐ |

**v4.0의 Go 선택 이유**:
- MoAI-ADK-Go 38,700줄 직접 재사용
- Go 커뮤니티 Rust 대비 2-3배
- 개발 속도 2-3배
- 단일 바이너리
- Go AI 생태계 성숙 (Eino, Genkit, ADK-Go)

**Trade-off 수용**:
- Rust 대비 성능 ~20% 손실
- GC 미세 지연 (UI에서 무시할만함)

### 4.2 시장 변화

| 버전 | 시장 | 비즈 모델 |
|-----|------|---------|
| v0.2 | Global | 없음 |
| v1.0 | Global | 마켓플레이스 |
| v2.0 | Global | 구독 |
| v3.0 | **Korea only** | **KT 매각** |
| **v4.0** | **Global** | **오픈소스 + 지원** |

### 4.3 수익 모델 변화

**v1.0-v2.0**: 마켓플레이스 + 구독 (BYOK 옵션)
**v3.0**: KT 매각 (One-time exit, 500-800억원)
**v4.0**: 
- 완전 오픈소스 MIT
- 선택적 Cloud 호스팅 서비스
- Enterprise 지원 계약
- GitHub Sponsors
- 컨설팅

---

## 5. 각 버전에서 물려받는 자산 (Asset Inheritance)

피벗해도 이전 작업이 헛되지 않음.

### 5.1 v0.2 → v1.0 자산
- Rust 크레이트 설계 원칙
- Claude Code 분석 자료 (계속 사용)

### 5.2 v1.0 → v2.0 자산
- World Fabric 컨셉 → A2A 프로토콜로 단순화
- 3-tier 에이전트 계층 (Super/Org/Worker) → 유지

### 5.3 v2.0 → v3.0 자산
- 듀얼 액세스 (PC + Mobile) → 유지
- 토큰 경제 구조 → 유지 (글로벌로 복귀)
- Tauri + React Native 결정 → 유지

### 5.4 v3.0 → v4.0 자산
- 프라이버시 아키텍처 (E2EE Relay) → 유지
- 다국어 지원 (한국어 포함) → 확장
- Privacy 규제 대응 → 글로벌 확장
- 사용자 적응 프레임워크 → 자기진화로 확장

### 5.5 v4.0의 총합 자산

**코드 자산**:
- MoAI-ADK-Go 38,700줄 (직접 계승)
- Claude Code 소스 분석 (747 파일)
- Hermes Agent 분석 (Python)

**설계 자산**:
- 5개 버전의 아키텍처 이터레이션
- 8가지 ADR 진화
- 브랜드/UX 원칙

**시장 자산**:
- 경쟁사 분석 (글로벌 + 한국)
- 규제 대응 방법
- 2026 산업 동향

---

## 6. v3.0 → v4.0 마이그레이션 가이드

### 6.1 제거되는 것 (v3.0 Korea 전용)

❌ **완전 제거**:
- KT 기가지니 포팅 (`genie-kt-adapter`)
- 지니뮤직 통합 (`genie-music`)
- 올레TV 통합 (`genie-iptv`)
- KT 매각 전략 문서 (`exit-strategy.md` 삭제됨)
- PASS 인증 전용
- KakaoPay 기본 → 플러그인으로 이동

⚠️ **옵션으로 변경**:
- Mi:dm 1순위 → Mi:dm 지원 (P3 우선순위)
- 한국 결제 (KakaoPay, TossPay) → 플러그인
- 한국 서비스 통합 → 플러그인

### 6.2 언어 변경 (Rust → Go)

**Naming 변경**:
- `genie-` prefix → `genie-` prefix
- Rust crates → Go packages

**구조 변경**:
- `crates/` → `internal/` + `pkg/`
- `Cargo.toml` → `go.mod`
- `napi-rs FFI` → `gRPC client-server`
- `tokio` → `goroutines + context`

**재작성 필요**:
- 코어 런타임 (Rust → Go, MoAI 기반)
- LLM 프로바이더 클라이언트
- WASM 샌드박스 (wasmtime-go)
- 데스크톱 자동화 (robotgo)
- 스토리지 레이어 (sqlx → go-sqlite3)

### 6.3 유지 가능한 자산

**TypeScript 클라이언트**:
- 변경 최소 (클라이언트는 언어 중립)
- gRPC 전환만 필요 (napi-rs 제거)

**Proto 스키마**:
- 대부분 재사용 가능
- 추가: learning.proto, identity.proto

**아키텍처 패턴**:
- 6-layer 아키텍처 → 5-layer 단순화
- Agent-first 유지
- 3-tier hierarchy 유지

**브랜드 원칙**:
- 메타포만 변경: 지니 → 거위
- UX 원칙 유지
- 접근성 유지

### 6.4 신규 작업 (v4.0만의)

✨ **완전 신규**:
- **learning-engine** 패키지 (MoAI SPEC-REFLECT 계승 + 확장)
- **identity** 패키지 (Graphiti 기반 Identity Graph)
- **lora** 패키지 (User-specific LoRA 훈련)
- **continual** 패키지 (EWC, LwF, Replay)
- **privacy** 패키지 (DP, FL, Secure Aggregation)
- **proactive** 패키지 (능동 에이전트)

---

## 7. 피벗 방지 방법 (미래를 위해)

5번의 피벗은 많다. v4.0 이후 피벗을 최소화하기 위한 원칙.

### 7.1 핵심 비전 고정 (Immutable)

**GENIE의 불변 핵심** (변경 X):
- ✅ 자기진화 개인화 AI
- ✅ 오픈소스 MIT
- ✅ Go + TypeScript
- ✅ 글로벌 커뮤니티
- ✅ 거위 메타포

### 7.2 변경 가능한 것 (Mutable)

**이것들은 피벗 아님**:
- 세부 기능 (plugin 추가/삭제)
- 우선순위 (Phase 1 내 재정렬)
- 파트너십 (LLM 프로바이더 추가)
- 지역 확장 (신규 언어)

### 7.3 진짜 피벗 기준

다음 중 **2개 이상 동시 변경** 시만 "피봇"으로 간주:

1. 타깃 시장 (사용자)
2. 핵심 가치 제안
3. 수익 모델
4. 기술 스택 주요 변경
5. 브랜드 identity

예: Phase 1에서 "Go → Rust" 바꾸는 것은 피봇 아님 (단일 변경).
하지만 "Go → Rust + 글로벌 → 한국"은 피봇.

### 7.4 RFC 프로세스

v4.0 이후 모든 중대 변경은 RFC 필요:
- 제안자: 창립 팀 또는 커뮤니티
- 리뷰: 최소 2주
- 투표: Maintainer 합의
- 기록: `/rfcs/` 디렉토리 영구 보관

---

## 8. 교훈 (Lessons Learned)

### 8.1 Good Decisions (계속 하자)

✅ **3개 프로젝트 분석**: Claude Code, Hermes, MoAI-ADK
→ 계속 가치 있는 참조

✅ **Privacy 중심 설계**: 모든 버전에서 일관
→ 글로벌 규제 대응 가능

✅ **TypeScript 클라이언트**: Ink/React 안정적
→ 유지

✅ **다국어 지원**: 한국어 + 영어 + 일본어 + 중국어
→ 글로벌 접근성

### 8.2 Bad Decisions (피하자)

❌ **v1.0 "메타버스"**: 너무 거창
→ 비전은 구체적이어야

❌ **v3.0 KT 매각**: 너무 낙관적
→ 한국 M&A 현실 무시

❌ **언어 5번 변경**: Rust ↔ Rust ↔ Go
→ 한 번 결정하면 버텨야

❌ **너무 빠른 피벗**: 12일 5번
→ 각 버전을 충분히 검증한 뒤 이동

### 8.3 미래 원칙

1. **"Why" 명확히 → "How" 결정**
   - 매번 "왜 이걸 만드는지" 질문

2. **시장 검증 → 방향 결정**
   - 가설만으로 피벗 X

3. **커뮤니티 > 단일 대기업**
   - 한 회사에 운명 맡기지 말 것

4. **MVP 먼저 → 확장**
   - 완벽한 설계보다 작동하는 프로토타입

5. **Long-term brand > short-term exit**
   - 10년 브랜드를 만들자

---

## 9. v4.0 출발점

### 9.1 핵심 자산 (Starting Assets)

**코드**:
- MoAI-ADK-Go 38,700줄 (직접 재사용)
- 3개 프로젝트 분석 자료
- v0.2 ~ v3.0 설계 문서 (재활용)

**브랜드**:
- GENIE 메타포 (새로움, 장기 가능)
- 거위 심볼리즘
- MIT 라이선스 정체성

**커뮤니티**:
- MoAI 기존 지지자
- Go 개발자 커뮤니티
- 오픈소스 기여자 풀

**지식**:
- 2026 AI 산업 동향 (최신)
- ICLR 2026 Lifelong Agents
- Self-evolving agents 연구

### 9.2 첫 번째 스프린트 (Phase 1)

**Month 1 (MVP 시작)**:
- Go 코어 포팅 (`genie-core`)
- MoAI-ADK SPEC-REFLECT 계승
- Basic CLI (`genie-cli`)

**Month 2-3 (확장)**:
- Identity Graph (Graphiti 통합)
- Basic Pattern Mining
- Desktop app (Tauri)

**Month 4-6 (알파)**:
- User LoRA (기초)
- Proactive Engine
- 1,000 beta users

### 9.3 Milestone 체크포인트

- **Month 3**: MVP (CLI + Desktop)
- **Month 6**: Alpha (학습 엔진 작동)
- **Month 9**: Beta (iOS/Android)
- **Month 12**: v1.0 official release
- **Month 18**: 플러그인 마켓 성숙
- **Month 24**: 10K+ 활성 사용자

---

## 10. 이해관계자 메시지

### 10.1 이전 v3.0 지지자 (있다면)

"KT 매각 전략은 야심찼지만 한국 M&A 현실과 맞지 않았습니다.
하지만 핵심 혁신 (자기진화, 개인화)은 유지되며,
오히려 글로벌 오픈소스로 더 큰 기회를 얻게 되었습니다.
여러분의 관심은 그대로 유효합니다."

### 10.2 새로운 v4.0 지지자

"안녕하세요, GENIE 커뮤니티에 오신 것을 환영합니다.
MIT 라이선스의 완전 오픈소스 AI 에이전트입니다.
자기진화 + 100% 개인화 = 평생 동반자 AI.
참여 방법은 github.com/genieagent/genie."

### 10.3 코드 기여자

"Go 학습이 필요하지만, MoAI-ADK-Go 38,700줄 기반이므로 러닝 커브 완만합니다.
First Issue 라벨로 시작하시면 좋습니다.
모든 기여자는 'Feather' 뱃지를 받습니다."

### 10.4 투자자 (엔젤)

"GitHub Sponsors 또는 Open Collective 통해 지원 가능.
전통적 VC 투자보다는 지속 가능한 오픈소스 모델.
Enterprise 지원 계약도 가능 (2027년 예정)."

### 10.5 학술 연구자

"GENIE는 자기진화 AI 연구의 open platform.
ICLR 2026 Lifelong Agents Workshop 참여 예정.
데이터셋 (익명) 공개 가능. Research@genieagent.org."

---

## 11. 솔직한 자기 평가

### 11.1 잘한 점

- 각 피벗마다 교훈을 학습했다
- 핵심 기술 아키텍처는 일관성 유지
- 3개 소스 프로젝트 분석은 계속 가치 있음
- 최종 v4.0 방향은 실제로 설득력 있음

### 11.2 못한 점

- **너무 많은 피벗** (12일 5번)
- 각 버전을 충분히 검증 안 함
- 사용자 리서치 부족
- 초기 "비전 명확화" 미흡

### 11.3 개선 방향

- v4.0 이후 피벗 금지
- 각 기능 MVP 먼저
- 커뮤니티 피드백 주기적
- 데이터 기반 의사결정

---

## 12. 결론

5번의 피벗을 거쳐 **GENIE**에 도착했다.

이제는 확고한 비전:
- **자기진화 개인화 AI**
- **글로벌 오픈소스 (MIT)**
- **Go + TypeScript**
- **거위 메타포 (평생 동반자)**
- **사용자 데이터 소유권**

v4.0은 다음에 바뀌지 않는다.
변할 수 있는 것은 **세부사항**이지, **비전**이 아니다.

**"Every day, a little more. For the rest of our lives together."**

---

## 부록: 각 버전의 핵심 파일 변화

### A. Document 변화

| 버전 | 주요 문서 | 핵심 파일 |
|-----|---------|---------|
| v0.2 | 없음 (초기) | 설계 노트만 |
| v1.0 | product.md v1.0 | 메타버스 비전 |
| v2.0 | 6개 문서 | product, structure, tech, branding, token-economy, ecosystem |
| v3.0 | 7개 문서 | + exit-strategy (KT 매각) |
| **v4.0** | **8개 문서** | **+ learning-engine, adaptation, migration (이 문서)** - exit-strategy |

### B. 프로젝트명 변화

- v0.2: `goos-agent-os`
- v1.0: `goos-agent-os` (Oz World)
- v2.0: `genie-agent` (글로벌)
- v3.0: `genie-kr` (한국)
- **v4.0: `genie-agent`** (글로벌)

### C. 타깃 매각가/가치

- v0.2: 없음
- v1.0: 마켓플레이스 수수료
- v2.0: 구독 기반
- v3.0: ₩500-800억 (KT 매각)
- **v4.0: 오픈소스 (화폐 가치 ≠ 주 목표)**

---

Version: 4.0.0
Created: 2026-04-21
Status: Final (v4.0은 변하지 않는다)

> **"5 pivots. 1 destination. GENIE."**
