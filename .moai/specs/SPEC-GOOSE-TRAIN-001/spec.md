---
id: SPEC-GOOSE-TRAIN-001
version: 0.1.0
status: planned
created_at: 2026-04-29
updated_at: 2026-05-17
author: manager-spec
priority: P2
issue_number: null
phase: 2
size: 대(L)
lifecycle: spec-first
labels: [training, ml, mlx, lora, rl, gemma4, phase-2]
target_milestone: v0.2.0
mvp_status: deferred
deferred_reason: "0.1.0 MVP 범위 외 — v0.2.0 이월 (2026-05-17 사용자 확정). RL 훈련 파이프라인은 GEMMA4-001 전환 이후 후순위, priority P1 → P2."
---

# SPEC-GOOSE-TRAIN-001 — MLX RL Training Pipeline for Gemma 4

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-29 | 초안 작성 (ROADMAP Phase 2, AI.MINK Gemma 4 RL training pipeline) | manager-spec |

---

## 1. 개요 (Overview)

AI.MINK의 **Gemma 4 E4B-IT 모델 기반 RL(Reinforcement Learning) 훈련 파이프라인**을 정의한다. M4 Max 128GB 환경에서 Apple MLX 생태계를 활용하여 SFT → DPO → GRPO 3단계 훈련을 수행하고, 훈련된 모델을 GGUF로 변환하여 Ollama 레지스트리에 배포하는 전체 과정을 규정한다.

수락 조건 통과 시점에서:

- `training/` 디렉터리에 SFT, DPO, GRPO 각 단계별 Python 스크립트가 존재한다.
- SFT 데이터는 한국어 대화(40%), AI.MINK tool call(20%), Socratic interview(15%), memory-referencing(15%), delegation routing(10%) 비율로 구성된 JSONL 파일이다.
- LoRA rank가 설정 가능하며(기본 16, 범위 8-64) 훈련 메트릭(loss, lr, examples/sec)이 stdout에 로깅된다.
- DPO는 SFT-finetuned LoRA adapter를 시작점으로 하여 preference optimization을 수행한다.
- GRPO는 tool call JSON schema compliance, Korean language quality, routing accuracy 보상 함수를 사용한다.
- LoRA adapter가 base model에 merge되어 full weights가 생성된다.
- Full weights가 FP16, Q4_K_M, Q5_K_M, Q8_0 포맷의 GGUF로 변환된다.
- GGUF 모델이 `ollama create` + `ollama push`로 Ollama 레지스트리에 배포된다.
- Export 시 Modelfile에 system prompt와 parameters가 자동 생성된다.

---

## 2. 배경 (Background)

### 2.1 왜 MLX + Gemma 4

- AI.MINK는 **로컬 우선** 정책(product.md §3.1)을 채택. M4 Max 128GB는 Gemma 4 E4B-IT의 full fine-tuning과 대형 LoRA 훈련이 가능한 하드웨어.
- MLX는 Apple Silicon에 최적화된 ML 프레임워크로, Metal GPU를 통한 훈련 가속을 제공. MLX-LM은 LoRA, DPO, GRPO 훈련을 네이티브 지원.
- Gemma 4 E4B-IT는 Google의 instruction-tuned 4B parameter 모델로, 한국어 이해도와 도구 호출 능력이 균형 잡힌 모델.
- 훈련된 모델을 GGUF로 변환하여 Ollama를 통해 크로스 플랫폼 배포 가능.

### 2.2 3단계 훈련 전략

1. **SFT (Supervised Fine-Tuning)**: AI.MINK 특화 태스크 수행 능력 부여. LoRA를 사용하여 base model에 한국어 대화·tool call·Socratic interview 등의 패턴을 학습.
2. **DPO (Direct Preference Optimization)**: SFT 모델을 기반으로 선호도 학습. chosen vs rejected 응답 쌍으로 AI.MINK 스타일의 응답 품질을 강화.
3. **GRPO (Group Relative Policy Optimization)**: 검증 가능한 보상 함수로 RL 수행. Tool call schema compliance, 한국어 품질, routing 정확도를 직접 최적화.

### 2.3 SPEC-GOOSE-LORA-001과의 관계

- SPEC-GOOSE-LORA-001은 **Go+Rust 기반 QLoRA 훈련 엔진**(온디바이스, user-specific LoRA adapter)을 규정.
- 본 SPEC(SPEC-GOOSE-TRAIN-001)은 **Python/MLX 기반의 base model RL 훈련**(서버 측, 단일 모델 배포용)을 규정.
- 두 SPEC은 독립적이며, LORA-001은 본 SPEC에서 훈련된 모델을 base로 사용할 수 있다.

### 2.4 범위 경계

- **IN**: SFT/DPO/GRPO 훈련 스크립트, 데이터 관리, LoRA merge, GGUF 변환, Ollama 배포, 훈련 메트릭 로깅, 설정 가능한 hyperparameter.
- **OUT**: 온디바이스 QLoRA(SPEC-GOOSE-LORA-001), 데이터 수집/라벨링 자동화, 분산 훈련, 훈련 UI/대시보드, 모델 평가 벤치크(GOOSE-Bench).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `training/sft/`: SFT 훈련 스크립트(`train_sft.py`) — MLX-LM LoRA 기반.
2. `training/dpo/`: DPO 훈련 스크립트(`train_dpo.py`) — MLX DPO 기반.
3. `training/grpo/`: GRPO 훈련 스크립트(`train_grpo.py`) — MLX GRPO 기반.
4. `training/data/sft/`: SFT 훈련 데이터(JSONL) — 버전 관리 대상.
5. `training/data/dpo/`: DPO 훈련 데이터(JSONL) — 버전 관리 대상.
6. `training/data/grpo/`: GRPO reward function 정의 + 테스트 데이터(JSONL, each line: `{"response": str, "expected_route": str, "tool_call_valid": bool, "korean_quality": float}`).
7. `training/configs/`: Hyperparameter YAML 설정 파일(단계별).
8. `training/export/`: LoRA merge + GGUF 변환 + Ollama 배포 스크립트.
9. `training/utils/`: 공통 유틸리티(데이터 검증, 메트릭 로깅, 체크포인트 관리).
10. `training/README.md`: 파이프라인 사용 가이드.

### 3.2 OUT OF SCOPE

- **온디바이스 QLoRA 훈련**: SPEC-GOOSE-LORA-001.
- **훈련 데이터 자동 수집/생성**: 후속 SPEC에서 사용자 trajectory 기반 데이터 파이프라인 구축.
- **분산 훈련**: M4 Max 단일 머신 기준. multi-node는 고려하지 않음.
- **훈련 UI/대시보드**: CLI 기반 stdout 로깅만. TensorBoard/Weights & Biases 연동은 후속 작업.
- **모델 평가 벤치마크**: GOOSE-Bench(후속 SPEC)가 담당.
- **Base model 다운로드/관리**: 사용자가 `huggingface-cli download`로 사전 준비한다고 가정.
- **MLX 프레임워크 자체 수정**: MLX/MLX-LM은 pip 설치하여 사용.

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-TR-001 [Ubiquitous]** — The training pipeline **shall** support SFT on Gemma 4 E4B-IT using MLX-LM LoRA with configurable LoRA rank (default: 16, range: 8-64). Values outside this range **shall** be rejected at config-load time with an `InvalidConfigError` listing the valid range.

**REQ-TR-002 [Ubiquitous]** — SFT training data **shall** consist of the following distribution: Korean dialogue (40%), AI.MINK tool calls (20%), Socratic interview patterns (15%), memory-referencing responses (15%), delegation routing (10%).

**REQ-TR-003 [Ubiquitous]** — SFT training data **shall** be stored in JSONL format with version control, where each line contains `{"messages": [{"role": str, "content": str}, ...]}`.

**REQ-TR-004 [Ubiquitous]** — LoRA rank, learning rate, batch size, number of epochs, and warmup steps **shall** be configurable via YAML configuration files per training stage. `lora_alpha` **shall** default to `2 * lora_rank` (following the LoRA convention where alpha=2*rank provides stable gradient scaling).

**REQ-TR-005 [Ubiquitous]** — Training **shall** log loss, learning rate, and examples/sec to stdout at configurable intervals (default: every 10 steps).

### 4.2 Event-Driven (이벤트 기반)

**REQ-TR-006 [Event-Driven]** — **When** a training run is invoked, the pipeline **shall** support DPO training on the SFT-finetuned LoRA adapter as the starting checkpoint.

**REQ-TR-007 [Event-Driven]** — **When** DPO training data is loaded, the pipeline **shall** accept the format `{"prompt": str, "chosen": str, "rejected": str}` in JSONL with one entry per line.

**REQ-TR-008 [Event-Driven]** — **When** the GRPO training stage is invoked, the pipeline **shall** apply reward functions including: (a) tool call JSON schema compliance score, (b) Korean language quality score, (c) routing accuracy score.

**REQ-TR-009 [Event-Driven]** — **When** GRPO is configured with a reference model, the pipeline **shall** compute KL divergence penalty against the reference model outputs.

**REQ-TR-010 [Event-Driven]** — **When** a training step completes, the pipeline **shall** save a checkpoint (adapter weights + optimizer state + training config) to the configured checkpoint directory.

### 4.3 State-Driven (상태 기반)

**REQ-TR-011 [State-Driven]** — **While** LoRA adapter weights exist from a previous stage, the next stage **shall** load them as the initial state rather than training from scratch.

**REQ-TR-012 [State-Driven]** — **While** GPU memory utilization exceeds 95%, the training script **shall** log a warning and suggest reducing batch size or LoRA rank.

### 4.4 Unwanted Behavior (방지)

**REQ-TR-013 [Unwanted]** — **If** training data fails schema validation (missing required fields, incorrect types), **then** the training script **shall** exit with a non-zero code and report the first N validation errors (default: 10) without starting training.

**REQ-TR-014 [Unwanted]** — The training pipeline **shall not** store model artifacts (LoRA adapters, merged weights, GGUF files) inside the git repository; all large artifacts **shall** be stored in a configurable external directory (default: `~/.goose/training-artifacts/`). The project `.gitignore` **shall** include the following patterns to prevent accidental commits: `*.gguf`, `*.safetensors`, `adapters/`, `checkpoints/`, `merged_weights/`, and the configured artifact directory path.

**REQ-TR-015 [Unwanted]** — The export script **shall not** delete the LoRA adapter after merging; both adapter and merged weights **shall** be preserved.

### 4.5 Export Pipeline

**REQ-TR-016 [Event-Driven]** — **When** the export pipeline is invoked, it **shall** merge the LoRA adapter into the base model producing full weights.

**REQ-TR-017 [Event-Driven]** — **When** full weights are available, the pipeline **shall** convert them to GGUF format supporting at minimum: FP16, Q4_K_M, Q5_K_M, Q8_0 quantization levels.

**REQ-TR-018 [Event-Driven]** — **When** a GGUF model file is available, the pipeline **shall** publish it to the Ollama registry via `ollama create` followed by `ollama push`.

**REQ-TR-019 [Event-Driven]** — **When** exporting to Ollama, the pipeline **shall** generate a Modelfile containing the AI.MINK system prompt and default inference parameters (temperature, top_p, context_length).

### 4.6 Tooling & Infrastructure

**REQ-TR-020 [Ubiquitous]** — SFT **shall** use `mlx_lm.lora --train` command or equivalent MLX-LM API.

**REQ-TR-021 [Ubiquitous]** — DPO **shall** use `mlx-lm-lora --dpo` or equivalent MLX DPO implementation.

**REQ-TR-022 [Ubiquitous]** — GRPO **shall** use `mlx-lm-lora --grpo` or equivalent MLX GRPO implementation.

**REQ-TR-023 [Ubiquitous]** — GGUF conversion **shall** use llama.cpp `convert_hf_to_gguf.py` script.

---

## 5. 수용 기준 (Acceptance Criteria)

**AC-TR-001 — SFT 훈련 실행**
- **Given** Gemma 4 E4B-IT base model이 로컬에 다운로드되어 있고, SFT JSONL 데이터가 `training/data/sft/train.jsonl`에 존재함
- **When** `python training/sft/train_sft.py --config training/configs/sft.yaml` 실행
- **Then** 훈련이 시작되고, stdout에 step별 loss, lr, examples/sec가 출력되며, 완료 후 LoRA adapter가 checkpoint 디렉토리에 저장됨

**AC-TR-002 — SFT 데이터 스키마 검증**
- **Given** JSONL 파일에 `{"messages": [{"role": "user", "content": "..."}]}` 형식의 데이터가 있음
- **When** 데이터 검증 스크립트 실행
- **Then** role이 system/user/assistant/tool 중 하나인지, content가 비어있지 않은지 검증하고, 오류 시 행 번호와 함께 보고

**AC-TR-003 — DPO 훈련 실행**
- **Given** SFT 훈련이 완료되어 LoRA adapter가 존재하고, DPO JSONL 데이터가 `training/data/dpo/train.jsonl`에 존재함
- **When** `python training/dpo/train_dpo.py --config training/configs/dpo.yaml --adapter <sft_adapter_path>` 실행
- **Then** SFT adapter를 초기 가중치로 로드하고 DPO 훈련이 수행됨

**AC-TR-004 — GRPO 보상 함수**
- **Given** GRPO 훈련 스크립트와 reward function 정의
- **When** GRPO 훈련 실행
- **Then** tool call JSON schema compliance, Korean language quality, routing accuracy 3개 보상이 개별적으로 계산되어 logged됨

**AC-TR-004a — KL Divergence Penalty** (verifies REQ-TR-009)
- **Given** GRPO config에 reference model 경로 설정
- **When** GRPO 훈련 step 실행
- **Then** KL divergence penalty가 계산되어 total reward에서 차감됨, 로그에 `kl_penalty` 값이 기록됨

**AC-TR-004b — GPU Memory Warning** (verifies REQ-TR-012)
- **Given** 훈련 중 GPU 메모리 사용률이 95% 초과
- **When** 훈련 step 완료 후 메모리 체크
- **Then** 경고 로그 출력 ("GPU memory > 95%, consider reducing batch_size or lora_rank")

**AC-TR-005 — LoRA Merge**
- **Given** 훈련된 LoRA adapter와 base model
- **When** `python training/export/merge_lora.py --base <model_path> --adapter <adapter_path> --output <output_path>` 실행
- **Then** full weights가 output 경로에 생성되고, LoRA adapter는 그대로 보존됨

**AC-TR-006 — GGUF 변환**
- **Given** merged full weights
- **When** `python training/export/convert_gguf.py --input <merged_path> --quantize Q4_K_M,Q5_K_M,Q8_0,FP16` 실행
- **Then** 4개 GGUF 파일이 각각 생성됨 (예: `ai-goose-gemma4-e4b-rl-v1-Q4_K_M.gguf`)

**AC-TR-007 — Ollama 배포**
- **Given** GGUF 파일과 Modelfile
- **When** `python training/export/publish_ollama.py --model ai-goose/gemma4-e4b-rl-v1 --gguf <path>` 실행
- **Then** `ollama create ai-goose/gemma4-e4b-rl-v1 -f Modelfile` 실행 후 `ollama push ai-goose/gemma4-e4b-rl-v1` 실행됨

**AC-TR-008 — Checkpoint 저장**
- **Given** 훈련이 진행 중
- **When** 설정된 checkpoint 간격(step 수) 도달
- **Then** adapter weights + optimizer state + config YAML이 checkpoint 디렉토리에 저장됨

**AC-TR-009 — 설정 가능한 Hyperparameter**
- **Given** `training/configs/sft.yaml`에 `lora_rank: 32`, `learning_rate: 1e-4`, `batch_size: 4`, `num_epochs: 3`이 설정됨
- **When** SFT 훈련 실행
- **Then** 로그에 설정된 값이 반영되어 출력됨

**AC-TR-010 — 아티팩트 외부 저장**
- **Given** 훈련 완료
- **When** 파일 시스템 확인
- **Then** LoRA adapter, merged weights, GGUF 파일이 모두 `~/.goose/training-artifacts/` 아래에 저장되고, git repository 내부에 대용량 파일이 없음

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 디렉터리 구조

```
training/
├── README.md                     # Pipeline usage guide
├── requirements.txt              # Python dependencies
├── configs/
│   ├── sft.yaml                  # SFT hyperparameters
│   ├── dpo.yaml                  # DPO hyperparameters
│   └── grpo.yaml                 # GRPO hyperparameters
├── sft/
│   ├── train_sft.py              # SFT training entry point
│   └── validate_data.py          # SFT data validation
├── dpo/
│   ├── train_dpo.py              # DPO training entry point
│   └── validate_data.py          # DPO data validation
├── grpo/
│   ├── train_grpo.py             # GRPO training entry point
│   └── rewards.py                # Reward function definitions
├── export/
│   ├── merge_lora.py             # LoRA adapter merge
│   ├── convert_gguf.py           # GGUF conversion
│   ├── publish_ollama.py         # Ollama registry publish
│   └── generate_modelfile.py     # Modelfile generation
├── data/
│   ├── sft/
│   │   ├── train.jsonl           # SFT training data
│   │   ├── val.jsonl             # SFT validation data
│   │   └── README.md             # Data format documentation
│   ├── dpo/
│   │   ├── train.jsonl           # DPO training data
│   │   ├── val.jsonl             # DPO validation data
│   │   └── README.md             # Data format documentation
│   └── grpo/
│       ├── test_cases.jsonl      # GRPO test scenarios
│       └── README.md             # Reward function documentation
└── utils/
    ├── data_utils.py             # Data loading, validation, splitting
    ├── logging_utils.py          # Metrics formatting, stdout logging
    ├── checkpoint_utils.py       # Checkpoint save/load
    └── config_utils.py           # YAML config loading with defaults
```

### 6.2 핵심 설정 스키마

```yaml
# configs/sft.yaml (예시)
model:
  name: "google/gemma-4-e4b-it"
  path: "~/.cache/huggingface/hub/models--google--gemma-4-e4b-it"

training:
  lora_rank: 16
  lora_alpha: 32
  lora_dropout: 0.05
  learning_rate: 2e-5
  batch_size: 4
  num_epochs: 3
  warmup_steps: 100
  max_seq_length: 2048
  checkpoint_every: 500
  log_every: 10

data:
  train: "training/data/sft/train.jsonl"
  validation: "training/data/sft/val.jsonl"

output:
  adapter_dir: "~/.goose/training-artifacts/sft-adapter"
  checkpoint_dir: "~/.goose/training-artifacts/sft-checkpoints"
```

### 6.3 데이터 형식

**SFT JSONL:**
```json
{"messages": [{"role": "system", "content": "You are AI.MINK..."}, {"role": "user", "content": "..."}, {"role": "assistant", "content": "..."}]}
```

**DPO JSONL:**
```json
{"prompt": "user query here", "chosen": "preferred response", "rejected": "less preferred response"}
```

**GRPO Reward Functions (rewards.py):**
```python
def tool_call_schema_compliance(response: str) -> float:
    """Score 0.0-1.0 based on JSON schema validity of tool calls."""

def korean_language_quality(response: str) -> float:
    """Score 0.0-1.0 based on Korean grammar, naturalness, honorifics."""

def routing_accuracy(response: str, expected_route: str) -> float:
    """Score 0.0 or 1.0 binary: does the response route to the correct provider."""
```

**GRPO Test Data (test_cases.jsonl):**
```json
{"response": "<tool_call {...}>", "expected_route": "ollama", "tool_call_valid": true, "korean_quality": 0.9}
```
- `response` (str): the model response to evaluate.
- `expected_route` (str): correct routing target (e.g., "ollama", "anthropic", "codex-cli").
- `tool_call_valid` (bool): whether any embedded tool call JSON is schema-valid.
- `korean_quality` (float 0.0-1.0): reference quality score for Korean text evaluation.

### 6.4 Export 파이프라인

```
1. merge_lora.py:
   - mlx_lm.fuse --base <model> --adapter <adapter> --output <merged>

2. convert_gguf.py:
   - python convert_hf_to_gguf.py <merged_path> --outfile <output.gguf>
   - llama-quantize for each quantization level

3. generate_modelfile.py:
   - Generate Modelfile with system prompt, parameters

4. publish_ollama.py:
   - ollama create ai-goose/gemma4-e4b-rl-v1 -f Modelfile
   - ollama push ai-goose/gemma4-e4b-rl-v1
```

### 6.5 TDD 진입

훈련 파이프라인은 Python 기반이므로 pytest 기반 검증:

1. **RED**: `test_validate_sft_data_valid` — 정상 JSONL 통과 확인
2. **RED**: `test_validate_sft_data_missing_field` — 필수 필드 누락 시 에러
3. **RED**: `test_validate_dpo_data_format` — DPO 포맷 검증
4. **RED**: `test_reward_tool_call_valid` — tool call schema compliance 점수
5. **RED**: `test_reward_korean_quality` — 한국어 품질 점수
6. **RED**: `test_reward_routing_accuracy` — routing 정확도 점수
7. **RED**: `test_config_loading_defaults` — YAML config 기본값 로딩
8. **RED**: `test_checkpoint_save_load` — checkpoint 저장/로드 round-trip
9. **RED**: `test_modelfile_generation` — Modelfile 올바른 생성
10. **GREEN**: 최소 구현
11. **REFACTOR**: 유틸리티 모듈 분리

### 6.6 TRUST 5 매핑

| 차원 | 달성 |
|-----|-----|
| Tested | pytest + 데이터 스키마 검증, reward function 단위 테스트 |
| Readable | Python docstring + type hints, YAML config self-documenting |
| Unified | ruff formatter, mypy type checking |
| Secured | training data에 PII 미포함, HuggingFace token 환경 변수 관리 |
| Trackable | 훈련 메트릭 stdout logging, checkpoint 버전 관리 |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 하드웨어 | M4 Max 128GB | Apple Silicon, Metal GPU |
| 선행 SPEC | SPEC-GOOSE-ROUTER-001 | Routing 로직 (GRPO reward function 참조) |
| 선행 SPEC | SPEC-GOOSE-LLM-001 | Ollama adapter (배포 후 검증) |
| 후속 SPEC | SPEC-GOOSE-LORA-001 | User-specific QLoRA (본 SPEC 모델을 base로 사용 가능) |
| 후속 SPEC | GOOSE-Bench | 모델 평가 벤치마크 |
| 외부 | MLX + MLX-LM | Apple ML framework (`pip install mlx-lm`) |
| 외부 | llama.cpp | GGUF 변환 (`convert_hf_to_gguf.py`) |
| 외부 | Ollama | 모델 배포 (`ollama create`, `ollama push`) |
| 외부 | HuggingFace Hub | Base model download (`huggingface-cli`) |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | MLX-LM GRPO 미지원 또는 불안정 | 중 | 높 | MLX-LM GRPO 지원 여부 사전 검증. 미지원 시 TRL(Transformers Reinforcement Learning) + MPS backend로 fallback |
| R2 | 4B 모델 + LoRA 훈련 시 M4 Max 메모리 부족 | 낮 | 높 | LoRA rank 조정, gradient checkpointing 활성화. E4B 모델은 4B parameter로 128GB에서 충분히 수용 가능 |
| R3 | GGUF 변환 시 품질 저하(양자화 손실) | 중 | 중 | FP16 기준으로 먼저 검증, Q4_K_M은 가장 균형 잡힌 옵션. 품질 비교 테스트 필요 |
| R4 | 한국어 품질 reward function의 주관성 | 높 | 중 | 보상 함수는 heuristics 기반(형태소 분석기 + 패턴 매칭)으로 시작, 후속 반복에서 모델 기반 평가로 전환 |
| R5 | MLX-LM API breaking change | 중 | 중 | requirements.txt에 버전 핀. MLX-LM은 빠르게 발전 중이므로 특정 버전 고정 |
| R6 | Ollama registry push 권한/인증 문제 | 낮 | 중 | `ollama push` 전 로그인 상태 확인 스크립트 포함. 개인 registry도 지원 |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서

- `.moai/project/tech.md` §9.6 온디바이스 추론 (Rust goose-ml)
- `.moai/project/product.md` §3.2 User-specific LoRA Adapters
- `.moai/specs/SPEC-GOOSE-ROUTER-001/spec.md` — Routing 로직 (GRPO reward 참조)
- `.moai/specs/SPEC-GOOSE-LLM-001/spec.md` — Ollama adapter

### 9.2 외부 참조

- MLX-LM: https://github.com/ml-explore/mlx-lm
- MLX: https://github.com/ml-explore/mlx
- llama.cpp GGUF conversion: https://github.com/ggerganov/llama.cpp
- Ollama CLI: https://github.com/ollama/ollama
- Gemma 4: https://huggingface.co/collections/google/gemma-4

### 9.3 훈련 방법론 참조

- DPO: Rafailov et al., "Direct Preference Optimization" (NeurIPS 2023)
- GRPO: Group Relative Policy Optimization for LLMs (2024)

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **온디바이스 QLoRA 훈련**을 포함하지 않는다. SPEC-GOOSE-LORA-001.
- 본 SPEC은 **훈련 데이터 자동 수집/생성 파이프라인**을 포함하지 않는다. 후속 SPEC.
- 본 SPEC은 **분산/멀티노드 훈련**을 지원하지 않는다. 단일 M4 Max 기준.
- 본 SPEC은 **훈련 UI/대시보드**(TensorBoard, W&B 등)를 포함하지 않는다. CLI + stdout만.
- 본 SPEC은 **모델 평가 벤치마크**(GOOSE-Bench)를 포함하지 않는다. 후속 SPEC.
- 본 SPEC은 **MLX 프레임워크 자체 수정**을 포함하지 않는다. pip install 그대로 사용.
- 본 SPEC은 **base model 다운로드 자동화**를 포함하지 않는다. 사용자 사전 준비 가정.
- 본 SPEC은 **GGUF 외의 배포 포맷**(ONNX, SafeTensors 직접 배포 등)을 포함하지 않는다.
- 본 SPEC은 **훈련된 모델의 실시간 서빙 인프라**를 포함하지 않는다. Ollama가 serving 담당.

---

**End of SPEC-GOOSE-TRAIN-001**
