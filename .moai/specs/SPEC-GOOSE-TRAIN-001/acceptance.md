---
id: SPEC-GOOSE-TRAIN-001
version: 0.1.0
spec: spec.md
---

# Acceptance Criteria — SPEC-GOOSE-TRAIN-001

## Acceptance Scenarios (Given/When/Then)

### AC-TR-001: SFT Training Execution

- **Given** Gemma 4 E4B-IT base model is downloaded locally and SFT JSONL data exists at `training/data/sft/train.jsonl`
- **When** `python training/sft/train_sft.py --config training/configs/sft.yaml` is executed
- **Then** training starts, stdout logs loss/lr/examples/sec per step, and LoRA adapter is saved to the configured output directory upon completion

### AC-TR-002: SFT Data Schema Validation

- **Given** a JSONL file with entries `{"messages": [{"role": "user", "content": "..."}]}`
- **When** `python training/sft/validate_data.py training/data/sft/train.jsonl` is executed
- **Then** each entry is validated for required `messages` array, valid `role` values (system/user/assistant/tool), non-empty `content`, and errors are reported with line numbers

### AC-TR-003: DPO Training with SFT Adapter

- **Given** SFT training has completed with a LoRA adapter at `<sft_adapter_path>` and DPO JSONL data exists
- **When** `python training/dpo/train_dpo.py --config training/configs/dpo.yaml --adapter <sft_adapter_path>` is executed
- **Then** DPO training loads the SFT adapter as initial weights and completes training, saving a new adapter

### AC-TR-004: GRPO Reward Functions

- **Given** GRPO training script and reward function definitions
- **When** GRPO training is executed with `--log-rewards` flag
- **Then** individual scores for tool_call_schema_compliance, korean_language_quality, and routing_accuracy are computed and logged per evaluation step

### AC-TR-005: LoRA Merge to Full Weights

- **Given** a trained LoRA adapter and the base model path
- **When** `python training/export/merge_lora.py --base <model_path> --adapter <adapter_path> --output <output_path>` is executed
- **Then** full weights are produced at the output path, and the original LoRA adapter remains intact at its location

### AC-TR-006: GGUF Multi-Quantization Conversion

- **Given** merged full weights
- **When** `python training/export/convert_gguf.py --input <merged_path> --quantize Q4_K_M,Q5_K_M,Q8_0,FP16` is executed
- **Then** four GGUF files are created with naming pattern `ai-goose-gemma4-e4b-rl-v1-{QUANT}.gguf`

### AC-TR-007: Ollama Registry Publish

- **Given** a GGUF file and a generated Modelfile
- **When** `python training/export/publish_ollama.py --model ai-goose/gemma4-e4b-rl-v1 --gguf <path>` is executed
- **Then** `ollama create` and `ollama push` commands are executed in sequence, and the model is available via `ollama run ai-goose/gemma4-e4b-rl-v1`

### AC-TR-008: Checkpoint Persistence

- **Given** training is in progress with `checkpoint_every: 500` configured
- **When** step 500 is completed
- **Then** a checkpoint directory containing adapter weights, optimizer state, and config YAML is created

### AC-TR-009: Hyperparameter Configuration

- **Given** `training/configs/sft.yaml` with `lora_rank: 32`, `learning_rate: 1e-4`, `batch_size: 4`, `num_epochs: 3`
- **When** SFT training is started
- **Then** the logged configuration summary matches the YAML values

### AC-TR-010: Artifacts Outside Repository

- **Given** training has completed
- **When** the git repository is inspected
- **Then** no files larger than 1MB exist under `training/` (excluding JSONL data files), and all model artifacts are in `~/.goose/training-artifacts/`

---

## Edge Cases

| Edge Case | Expected Behavior |
|-----------|------------------|
| Empty JSONL file (0 lines) | Validation reports "0 entries found" and exits with error |
| JSONL with malformed JSON on line N | Validation reports line N with parse error, exits non-zero |
| LoRA rank = 8 (minimum) | Training proceeds normally with smaller adapter |
| LoRA rank = 64 (maximum) | Training proceeds with higher memory usage |
| GGUF conversion for unsupported quant | Script reports unsupported quantization and skips it |
| Ollama not running | publish script detects and reports "Ollama daemon not running" |
| Ollama not logged in | publish script reports authentication error with login instructions |
| GPU memory > 95% during training | Warning logged suggesting batch size or rank reduction |
| Missing base model path | Script exits with "Base model not found at <path>" error |

---

## Quality Gates

### TRUST 5 Validation

| Dimension | Criteria |
|-----------|----------|
| **Tested** | All reward functions have unit tests. Data validation has tests. Config loading has tests. pytest passes. |
| **Readable** | All Python files have docstrings. Type hints on all public functions. ruff passes with zero warnings. |
| **Unified** | ruff format applied. Consistent naming (snake_case). YAML config keys follow consistent convention. |
| **Secured** | No API keys or tokens in source code. HuggingFace token read from environment variable only. Training data contains no PII. |
| **Trackable** | Training metrics logged with step number. Checkpoints contain config snapshot. Export artifacts include version metadata. |

### Definition of Done

- [ ] All 10 acceptance scenarios pass
- [ ] All edge cases handled gracefully
- [ ] `pytest` passes with 0 failures
- [ ] `ruff check` passes with 0 errors
- [ ] No files > 1MB in git repository (excluding JSONL data)
- [ ] `training/README.md` documents full pipeline usage
- [ ] End-to-end dry-run test passes (mock MLX, verify file creation)
- [ ] Artifacts correctly stored in external directory

---

Version: 0.1.0
Last Updated: 2026-04-29
