---
id: SPEC-GOOSE-TRAIN-001
version: 0.1.0
spec: spec.md
---

# Implementation Plan — SPEC-GOOSE-TRAIN-001

## Milestones

### M1: 프로젝트 구조 + 설정 체계 (Priority: High)

- `training/` 디렉터리 구조 생성
- `requirements.txt` 작성 (mlx, mlx-lm, transformers, pytest, pyyaml, ruff)
- `training/configs/sft.yaml`, `dpo.yaml`, `grpo.yaml` 기본 설정 파일
- `training/utils/config_utils.py` — YAML 로딩 + 기본값 병합 + 검증
- `training/utils/logging_utils.py` — stdout 메트릭 포맷팅

### M2: 데이터 관리 + 검증 (Priority: High)

- `training/utils/data_utils.py` — JSONL 로딩, splitting, 통계
- `training/sft/validate_data.py` — SFT 데이터 스키마 검증 (messages, role, content)
- `training/dpo/validate_data.py` — DPO 데이터 스키마 검증 (prompt, chosen, rejected)
- `training/data/sft/README.md` — 데이터 형식 문서화
- `training/data/dpo/README.md` — 데이터 형식 문서화
- `training/data/grpo/README.md` — reward function 문서화
- 테스트: `tests/test_data_validation.py`

### M3: SFT 훈련 (Priority: High)

- `training/sft/train_sft.py` — MLX-LM LoRA SFT 진입점
- `training/utils/checkpoint_utils.py` — checkpoint save/load
- 훈련 메트릭 stdout 로깅 (loss, lr, examples/sec)
- LoRA rank 설정 가능 (YAML config)
- 테스트: `tests/test_sft_train.py` (dry-run 모드로 1 step)

### M4: DPO 훈련 (Priority: Medium)

- `training/dpo/train_dpo.py` — MLX DPO 진입점
- SFT adapter를 초기 가중치로 로드
- DPO 데이터 포맷 로딩 + 검증
- 테스트: `tests/test_dpo_train.py`

### M5: GRPO 훈련 + 보상 함수 (Priority: Medium)

- `training/grpo/rewards.py` — 3개 reward function 구현
  - tool_call_schema_compliance
  - korean_language_quality
  - routing_accuracy
- `training/grpo/train_grpo.py` — MLX GRPO 진입점
- Reference model KL divergence penalty 지원
- 테스트: `tests/test_rewards.py`, `tests/test_grpo_train.py`

### M6: Export 파이프라인 (Priority: Medium)

- `training/export/merge_lora.py` — LoRA adapter → full weights merge
- `training/export/convert_gguf.py` — GGUF 변환 (FP16, Q4_K_M, Q5_K_M, Q8_0)
- `training/export/generate_modelfile.py` — Modelfile 자동 생성
- `training/export/publish_ollama.py` — Ollama registry 배포
- 테스트: `tests/test_export.py`

### M7: 통합 + 문서화 (Priority: Low)

- End-to-end 파이프라인 dry-run 테스트
- `training/README.md` 작성
- 아티팩트 외부 저장 검증 (git 내부 대용량 파일 부재)
- `.gitignore`에 아티팩트 경로 추가

---

## Technical Approach

### 언어 & 프레임워크

- **Python 3.11+**: MLX 요구사항
- **MLX + MLX-LM**: Apple Silicon 최적화 훈련
- **PyYAML**: 설정 관리
- **pytest**: 테스트
- **ruff**: 린터/포매터

### 훈련 하드웨어 요구사항

- Apple M4 Max, 128GB Unified Memory
- macOS 15.0+
- Metal GPU 지원

### 데이터 버전 관리

- `training/data/` 하위 JSONL 파일은 git으로 버전 관리
- 대용량 모델 아티팩트는 `~/.goose/training-artifacts/` 에 저장
- `.gitignore`에 `*.gguf`, `*.safetensors`, `*.bin` 패턴 추가

### 아티팩트 디렉터리 구조

```
~/.goose/training-artifacts/
├── sft-checkpoints/
│   ├── checkpoint-500/
│   ├── checkpoint-1000/
│   └── final/
├── dpo-checkpoints/
│   └── ...
├── grpo-checkpoints/
│   └── ...
├── merged/
│   └── ai-goose-gemma4-e4b-rl-v1/
└── gguf/
    ├── ai-goose-gemma4-e4b-rl-v1-FP16.gguf
    ├── ai-goose-gemma4-e4b-rl-v1-Q4_K_M.gguf
    ├── ai-goose-gemma4-e4b-rl-v1-Q5_K_M.gguf
    └── ai-goose-gemma4-e4b-rl-v1-Q8_0.gguf
```

---

## Risks

| Risk | Mitigation |
|------|-----------|
| MLX-LM GRPO 미지원 | M5 시작 전 MLX-LM GRPO 지원 확인. 미지원 시 TRL + MPS fallback 경로 설계 |
| 훈련 데이터 부족 | SFT 데이터는 수작업 + 합성 생성 혼합. 최소 1,000개 대화로 시작 |
| GGUF 변환 실패 | llama.cpp 버전 호환성 확인. HuggingFace -> GGUF 변환 경로 사전 검증 |

---

## Dependencies

- M2 depends on M1
- M3 depends on M1, M2
- M4 depends on M3 (needs SFT adapter)
- M5 depends on M4 (needs DPO-finetuned adapter, optionally)
- M6 depends on M3 or M5 (needs trained adapter)
- M7 depends on all prior milestones

---

Version: 0.1.0
Last Updated: 2026-04-29
