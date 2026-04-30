## SPEC-GOOSE-ADAPTER-001-AMEND-001 Progress

- Started: 2026-04-30
- **Status: PLANNED (Phase 1 — plan phase 진행 중)**
- Parent SPEC: SPEC-GOOSE-ADAPTER-001 v1.0.0 (FROZEN, completed 2026-04-27)
- Author (Phase 1 draft): manager-spec
- Harness 후보: standard (단일 도메인 amendment, 신규 surface 추가 없음, 신규 LoC ~+130 production / ~+400 tests)
- Mode 후보: TDD (RED-GREEN-REFACTOR — 신규 매트릭스 24 케이스 + 통합 테스트 3건이 명확히 정의됨)
- Effort 후보: medium (Opus 4.7 default, reasoning-light 매트릭스 작업)

### Phase 1 — Plan phase memo (초안 작성 완료)

- 산출물:
  - `.moai/specs/SPEC-GOOSE-ADAPTER-001-AMEND-001/research.md` — 6 어댑터 × 2 기능 매트릭스 + provider 공식 문서 verified 링크 + 기존 코드 영향 분석 (변경 파일 목록 + LoC 추정 + R1~R4 risks)
  - `.moai/specs/SPEC-GOOSE-ADAPTER-001-AMEND-001/spec.md` — frontmatter (parent_spec/parent_version 포함) + EARS 12 REQ + Given/When/Then 11 AC + Exclusions 9 항목
  - `.moai/specs/SPEC-GOOSE-ADAPTER-001-AMEND-001/progress.md` (본 파일)
- 부모 surface 보존 원칙: HARD 4건 (interface 시그니처, CompletionRequest 필드, Capabilities zero-value 호환, 부모 24 테스트 0건 수정)
- Capability gate 위치: `internal/llm/provider/llm_call.go` 단일 처리(연구 §2.2 결정). 어댑터별 분산 처리 회피.

### Provider 매트릭스 (research.md §2~§3 결정 반영)

| Provider | JSONMode | UserID | 결정 근거 |
|----------|----------|--------|----------|
| anthropic | false | true | response_format 미지원, metadata.user_id 공식 지원 |
| openai | true | true | response_format json_object + user 표준 |
| xai | true | true | OpenAI 호환, user 명시 지원 |
| deepseek | true | false | response_format 명시 지원, user top-level 미문서 → silent drop |
| google | true | false | generationConfig.responseMimeType 지원, user identifier 부재 |
| ollama | true | false | format:"json" 지원, 로컬 LLM이라 user 불필요 |

### REQ/AC count

- 신규 REQ: 12 (REQ-AMEND-001 ~ REQ-AMEND-012, EARS 5축 분포: Ubiquitous 2, Event-Driven 6, State-Driven 1, Unwanted 2, Optional 1)
- 신규 AC: 11 (AC-AMEND-001 ~ AC-AMEND-011, REQ↔AC 매핑 표 §5 말미)
- Exclusions 항목: 9개 명시(부모 OUT 승계 + amendment 고유 8건)

### 다음 단계 권장

1. **plan-auditor 실행 권장** — 본 spec.md가 부모 SPEC FROZEN 정신을 준수하는지(특히 §3.2 surface 보존 4건 HARD 규칙), AC가 EARS REQ와 1:1 매핑되는지, parent_spec frontmatter가 의도한 amendment lineage를 정확히 표현하는지 독립 검증 필요. plan-auditor 실행 결과 GREEN이면 Phase 2(annotation cycle 또는 즉시 run) 진행.
2. **annotation cycle 1회** — 사용자가 (a) capability gate 단일/분산 처리 결정, (b) DeepSeek `user` field silent drop vs forwarding 시도, (c) Anthropic `output_config.format=json_schema` 별도 SPEC 분리 여부의 3개 결정 사항을 최종 확정.
3. **`/moai run SPEC-GOOSE-ADAPTER-001-AMEND-001`** — annotation 통과 후 manager-tdd 또는 manager-ddd로 위임 (TDD 권장 — 매트릭스 테스트 24건이 RED→GREEN 사이클로 명확히 정의됨).

### Open Questions (annotation cycle에서 사용자 확정 필요)

1. capability gate를 `llm_call.go`에 단일 처리할지, 어댑터별 분산할지 — research.md §6.1 R3 mitigation 비용 비교
2. DeepSeek `user` field 미문서 처리 — silent drop(권장) vs OpenAI 호환 가정 forwarding
3. Anthropic structured output(`output_config.format=json_schema`)을 본 amendment에 포함할지 — 권장: OUT(별도 SPEC)
4. UserID redaction 패턴 — 첫 4글자 + `...`(권장) vs 해시 vs 완전 제거
