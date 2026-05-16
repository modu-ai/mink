---
id: ADR-001
title: On-device QLoRA / RL training 노선 폐기 결정
status: accepted
date: 2026-05-16
deciders: [GOOS행님, MoAI orchestrator]
affects:
  - SPEC-GOOSE-LORA-001
  - SPEC-GOOSE-TRAIN-001
supersedes: []
superseded_by: []
---

# ADR-001: On-device QLoRA / RL Training 노선 폐기

## 1. Context

GOOSE/MINK 초기 비전(2026-04 기준 ROADMAP Phase 2 + Phase 6)은 사용자별 자기진화 LLM 을 다음 두 축으로 구현하려 했다.

- **SPEC-GOOSE-LORA-001**: 사용자별 QLoRA adapter (10~200 MB) on-device 학습 + Go 인터페이스 + Rust `goose-ml` crate 위임
- **SPEC-GOOSE-TRAIN-001**: M4 Max 128GB 기반 MLX 생태계에서 Gemma 4 E4B-IT 모델의 SFT → DPO → GRPO 3 단계 RL 파이프라인 + GGUF 변환 + Ollama 배포

본 노선은 "모델 가중치 자체가 사용자를 닮아간다"는 강력한 차별 메시지를 가졌지만, 동시에 다음 위험을 동반했다.

1. **학습 자원 부담**: M-series 노트북/맥미니에서 분당 수십 분~수시간 단위 학습 사이클이 일상화. 저사양 디바이스(M1 8GB / Windows / Linux 노트북)에서 사실상 불가.
2. **Catastrophic forgetting 및 safety drift**: 사용자 corpus 가 base 모델의 일반 능력·안전 정렬을 잠식할 위험. Canary + safety regression 평가 게이트 신설 필요 → 운영 복잡도 ↑.
3. **데이터 편향 흡수**: 부정적/위험 발화가 adapter 가중치에 흡수되면 추적·삭제·revoke 가 *모델 재학습* 을 요구. Markdown source-of-truth 의 *한 줄 편집* 단순성을 잃음.
4. **Polyglot 비용**: Rust `goose-ml` crate 도입 시 Go + Rust + cgo/uniffi 경계 관리, CI 매트릭스 확장, 컨트리뷰터 진입 장벽 2배.
5. **시장 표준과의 괴리**: OpenClaw / Hermes Agent / Claude Code 의 *self-improving* 은 실제로 **skill 자동 생성 + 메모리 진화** 차원이며 *가중치 학습이 아니다*. Hermes 의 Curator (7-day grade/consolidate/prune) 도 외부 메모리 작업이다. 즉 시장 표준은 "모델 외부에서 진화" 로 수렴.
6. **외부 GOAT LLM 의 품질·속도 진보**: Claude Opus 4.7, GPT-5.5, GLM-5-Turbo, DeepSeek 등의 외부 모델이 단일 사용자가 QLoRA 로 도달 가능한 품질을 매월 갱신. 자체 학습의 ROI 가 시간이 지날수록 감소.

## 2. Decision

다음 두 노선을 폐기한다.

- **SPEC-GOOSE-LORA-001**: status `planned` → `deprecated`
- **SPEC-GOOSE-TRAIN-001**: status `planned` → `deprecated`

대체 노선:

- **SPEC-MINK-MEMORY-QMD-001** (Go-native QMD 메모리 엔진) — sqlite-vec + FTS5 hybrid retrieval (vector 70 / BM25 30) + MMR + temporal decay (30일 half-life), ollama embed sidecar, 세션 transcript export
- **SPEC-MINK-LLM-ROUTING-V2-AMEND-001** (5-provider 외부 LLM 라우팅) — Anthropic Claude · DeepSeek · OpenAI GPT (API mode) · Codex (ChatGPT OAuth) · z.ai GLM-5-Turbo
- **SPEC-MINK-AUTH-CREDENTIAL-001** (keyring default + 평문 fallback)

## 3. Consequences

### 3.1 긍정적

- **운영 단순성 ↑**: 모델 호스팅 0, 학습 사이클 0, GPU/NPU 의존 0. 맥미니 M2 16GB 에서 안정 작동.
- **안전성 ↑**: 가중치 변경 없음 → catastrophic forgetting / safety drift 위험 제거. 사용자 데이터 삭제 = Markdown 파일 삭제 한 줄.
- **추적성 ↑**: 사용자가 *왜 그렇게 답했는지* 검색된 chunk 로 100% 설명 가능. QLoRA 의 가중치 블랙박스와 대조.
- **Go 단일 유지**: Rust `goose-ml` 도입 회피, polyglot 비용 0. v0.5.0 launch 까지 Go 단일 결정 (별도 ADR 또는 사용자 결정 메모).
- **시장 표준 흡수**: OpenClaw QMD / ClawMem vault 호환 → OpenClaw·Hermes·Claude Code 사용자 마이그레이션 경로 확보.

### 3.2 부정적 / 트레이드오프

- **"모델이 당신을 닮아간다"** 라는 비전적 카피의 *literal* 의미 상실. 단, *기억의 깊이 누적 + system prompt + retrieved context* 로 동일한 사용자 결의 응답을 달성 가능. 새 슬로건 *"Remembers you. Evolves with you."* 가 이 변화를 정확히 반영.
- 외부 LLM 의존 → 네트워크·서비스 가용성·요금에 노출. mitigation: 사용자가 자기 OAuth/API key 보유, 비용은 사용자 부담, 5 provider 중 다중화로 단일 장애 회피.
- 자체 모델 호스팅이 가능한 환경(GPU 서버, 기업 on-prem)에서 *Mink* 가 그 가치를 활용 못 함. mitigation: SPEC-MINK-LLM-ROUTING-V2-AMEND-001 의 "사용자 정의 OpenAI-compatible endpoint" 옵션으로 사용자가 자체 vLLM/Ollama/lm-studio endpoint 를 등록 가능.

### 3.3 마이그레이션

- LORA-001 / TRAIN-001 spec.md 본문은 역사 보존을 위해 그대로 유지하되 frontmatter `status: deprecated`, `superseded_by` 필드 명시.
- ROADMAP.md / IMPLEMENTATION-ORDER.md / PRODUCT-V7-001 / DISTANCING-STATEMENT-001 의 QLoRA·RL 학습 언급 부분은 후속 PR (Sprint 3 후반 또는 Sprint 5 hygiene) 에서 일괄 amendment.
- 코드 디렉토리 `internal/ml/*` (만약 stub 이 있다면) 는 *deprecated SPEC* 와 함께 정리. 현재 디스크 상태로는 미구현이므로 추가 정리 작업 없음 (검증: `git grep -l "QLoRA\|GRPO\|MLX" internal/` → 결과 없음 확인 필요).

## 4. Alternatives Considered

| 대안 | 채택 안 한 이유 |
|---|---|
| QLoRA 유지 + Hermes 식 *skill 진화* 만 추가 | 두 진화 채널 동시 운영의 안전 복잡도 ↑, ROI 낮음 |
| 자체 모델 호스팅 (vLLM on M4 Max) | 사용자 디바이스 자원 의존, 일반 사용자 접근성 ↓ |
| QLoRA 옵션화 (default off) | 코드·문서 부담 유지, 사용자 혼란 |
| Hermes 식 Curator 만 도입 (skill 자동 생성) | MoAI 의 `builder-harness` agent 가 이미 동일 역할. 별도 도입 불필요 |

## 5. References

- 사용자 결정 메시지 (Conversation 2026-05-16): "강화 학습 LLM 불필요", "QMD 를 이용해서 메모리를 구축", "기존의 GOAT LLM 을 사용"
- handoff_session_2026-05-12 ("IDEA-002 MINK brain") — 초기 brain plan 단계 기록, 본 ADR 로 supersede
- OpenClaw QMD memory engine docs: https://docs.openclaw.ai/concepts/memory-qmd
- ClawMem cross-agent vault: https://github.com/yoloshii/ClawMem
- Z.AI GLM-5-Turbo docs: https://docs.z.ai/guides/llm/glm-5
- Codex CLI Authentication: https://developers.openai.com/codex/auth
