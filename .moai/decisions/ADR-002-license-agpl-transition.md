---
id: ADR-002
title: 라이선스 전환 Apache-2.0 → GNU AGPL-3.0-only
status: accepted
date: 2026-05-16
deciders: [GOOS행님, MoAI orchestrator]
affects:
  - LICENSE
  - NOTICE
  - README.md
  - 모든 신규 코드 SPDX 헤더
  - CONTRIBUTING (있는 경우)
supersedes: []
superseded_by: []
related:
  - ADR-001 (QLoRA/RL 폐기) — 사용자 데이터 보호 강화 맥락 공유
---

# ADR-002: 라이선스 Apache-2.0 → GNU AGPL-3.0-only 전환

## 1. Context

MINK 는 modu-ai/mink 레포의 Go 단일 self-hosted lifelong AI companion 으로, 2026-04-27 부터 Apache License 2.0 으로 공개되어 왔다. 2026-05-16 사용자 결정으로 다음과 같이 라이선스를 전환한다.

### 1.1 현황 (2026-05-16 시점)

- LICENSE 파일: Apache License 2.0 전문 (~11.3 KB)
- README badge: `License: Apache 2.0`
- NOTICE: Apache-2.0 명시
- 코드 .go 파일: SPDX-License-Identifier 헤더 없음 (검증: `grep -rl SPDX-License-Identifier --include="*.go"` 0건)
- go.mod metadata: 라이선스 필드 명시 없음
- 의존성: 대부분 MIT/BSD-2/BSD-3/Apache-2.0/MPL-2.0 (AGPL-3.0 흡수 가능)

### 1.2 변화 동기

1. **자기진화 agent 의 사용자 권리 보장**: Mink 는 사용자 데이터(대화·journal·ritual)를 누적·인덱스(QMD)·검색(LLM 호출)에 활용한다. 만약 3rd party 가 Mink 를 SaaS 로 호스팅하여 다중 사용자를 받게 되면, MIT/Apache 라이선스로는 *서비스 측 소스 공개* 의무가 발생하지 않아 사용자가 자기 데이터·자기 agent 의 소스에 대한 권리를 잃을 수 있다. AGPL-3.0 의 **§13 (Remote Network Interaction)** 이 이 빈틈을 차단한다.
2. **Hermes Agent / OpenClaw 와의 차별화**: Hermes 는 MIT, OpenClaw 는 사실상 MIT/Apache. 두 프로젝트 모두 *데이터 권리* 측면에서 AGPL 보다 약하다. Mink 가 AGPL-3.0 을 채택하면 *"Your data, your model, your source — even when we host it"* 라는 한 줄짜리 GTM 차별점 확보.
3. **엔터프라이즈 채택 매트릭스**: AGPL-3.0 은 *오용 방지* 가 강하므로 *클라우드 벤더의 wrap-and-sell* 우려를 줄인다. 단, 일부 기업의 OSS 정책에서 AGPL 사용 금지 사례가 있어 채택률 트레이드오프 존재 (§3.2 부정적 효과 참조).
4. **MoAI-ADK 와의 정합**: 같은 modu-ai org 의 moai-adk 가 Apache-2.0 이지만, *applet 성격* 의 도구와 *lifelong agent* 인 Mink 의 거버넌스 요구는 다르다. 두 프로젝트 라이선스 분리는 합리적.

### 1.3 "Copyleft 3.0" 사용자 결정 해석

사용자 표현 "Copyleft 3.0" 의 정확한 식별자는 SPDX `AGPL-3.0-only` 로 확정 (2026-05-16 Round 2 AskUserQuestion 답변). 대안 `GPL-3.0-only` 는 SaaS 우회 빈틈으로 거부, `LGPL-3.0-only` 는 라이브러리가 아니므로 부적합.

## 2. Decision

다음 4 영역에서 라이선스를 Apache-2.0 → AGPL-3.0-only 로 전환한다.

| 영역 | 본 PR 에서 처리 | 후속 PR 로 이월 |
|---|---|---|
| LICENSE 파일 | ✅ 전문 교체 (AGPL-3.0 표준 텍스트, 34,527 bytes) | — |
| NOTICE | ✅ AGPL-3.0 명시 + 2026-05-16 전환 노트 + 의존성 호환성 cross-link | — |
| README badge / 본문 4 곳 | ✅ 모두 갱신 | — |
| `.moai/decisions/ADR-002` | ✅ 본 파일 | — |
| .go 파일 SPDX 헤더 | — | **별도 PR**: 전체 .go 파일에 `// SPDX-License-Identifier: AGPL-3.0-only` 일괄 삽입 (expert-refactoring codemod) |
| go.mod license metadata | — | **별도 PR**: 필요 시 (Go 표준 go.mod 에는 license 필드 없음, README 의존) |
| CONTRIBUTING.md DCO/CLA | — | **별도 PR**: 만약 CONTRIBUTING.md 가 존재하면 갱신, 없으면 신규 작성 |
| 의존성 호환성 audit | — | **별도 PR**: `go-licenses` 또는 동등 도구로 전체 모듈 라이선스 매트릭스 검증 |

## 3. Consequences

### 3.1 긍정적

- **§13 네트워크 사용 조항**: SaaS 호스팅 시에도 사용자가 *수정된 소스* 에 접근 가능. 사용자 데이터·agent 소스에 대한 권리 종단 보장
- **차별화 메시지**: Hermes(MIT) / OpenClaw 와의 라이선스 우위 마케팅 가능
- **특허 grant 포함**: AGPL-3.0 도 §11 에서 특허 grant 보유 (Apache-2.0 보다 좁지만 존재)
- **호환성**: Apache-2.0 코드는 AGPL-3.0 으로 흡수 가능 (Apache→AGPL 일방 호환). 기존 의존성 영향 0
- **기존 commits 의 라이선스 안정성**: 2026-05-16 이전 commits 는 git history 에 그대로 보존, 원래 Apache-2.0 조건 유지

### 3.2 부정적 / 트레이드오프

- **기업 채택 트레이드오프**: 일부 기업은 OSS 정책으로 AGPL 사용 금지 (Google 의 일부 부서, AWS 의 일부 정책 등). 그러나 Mink 의 1차 타깃은 *개인 사용자·소규모 팀* 이므로 영향 제한적
- **컨트리뷰터 CLA 검토**: 향후 외부 컨트리뷰션 받을 때 AGPL 약정 명시 필요 (CONTRIBUTING.md 후속 PR)
- **plugins/extensions 라이선스 호환성**: Mink 가 외부 플러그인 marketplace 도입 시 플러그인 라이선스가 AGPL 와 호환되어야 함. 후속 SPEC 에서 명시
- **Anthropic Claude / OpenAI / DeepSeek / z.ai API SDK 라이선스**: 모두 MIT 또는 Apache-2.0 → AGPL 흡수 가능. *API 사용* 자체는 라이선스 호환과 무관 (단순 네트워크 호출)
- **ollama**: MIT — 호환
- **sqlite-vec (asg017/sqlite-vec)**: Apache-2.0 → 호환
- **mattn/go-sqlite3**: MIT → 호환
- **Bubble Tea / cobra / go-keyring**: MIT/Apache-2.0 → 호환

### 3.3 사용자 경험 변화

- 외부 사용자 입장에서는 **변화 없음**. 본인 디바이스에서 self-host 하는 한 AGPL §13 가 트리거되지 않음 (네트워크 다중 사용자 호스팅 시에만 의무 발생)
- *3rd party 가 Mink 를 SaaS 로 운영* 하는 경우 그 사업자가 *수정 소스 공개* 의무 가짐 → 사용자가 그 fork 를 받아 self-host 전환 가능
- 컨트리뷰터 입장에서는 *AGPL 약정* 명시 동의가 필요 (PR 본문 또는 CONTRIBUTING)

## 4. Dependency License Audit (후속 PR 대상)

본 PR 시점에서는 audit 미수행. 후속 PR 에서 `go-licenses report ./...` 또는 동등 도구로 다음 매트릭스 작성:

| 의존성 | 라이선스 | AGPL-3.0 호환 |
|---|---|---|
| (자동 생성) | (자동 생성) | (자동 생성) |

알려진 위험 라이선스 (AGPL 와 비호환 가능성):
- BUSL (Business Source License) — 일부 데이터베이스 제품 (MariaDB, CockroachDB 일부 버전)
- SSPL (Server Side Public License) — MongoDB, Elastic 등
- Confluent Community License — Kafka 일부
- Commons Clause — 일부 redis fork

현 시점 Mink 의존성에 위 라이선스 없음 (수동 검증 필요).

## 5. Migration Notes

### 5.1 외부 컨트리뷰터

- 본 PR 머지 이후의 모든 컨트리뷰션은 AGPL-3.0-only 약정 하에 받는다
- 기존 PR (open 상태) 은 컨트리뷰터에게 통지 후 동의 받음
- CONTRIBUTING.md 가 신설되면 DCO sign-off 또는 명시적 약정 요구

### 5.2 fork 사용자

- 본 PR 이전 commit 을 base 로 한 fork 는 Apache-2.0 조건 유지 가능
- 본 PR 이후 commit 을 흡수한 fork 는 자동 AGPL-3.0-only

### 5.3 패키지 배포

- 향후 binary 배포 시 LICENSE + NOTICE 동봉 의무 (AGPL-3.0 §4 source 제공 또는 written offer)
- Homebrew formula / apt package / chocolatey 등 패키지 매니저 채널 시 라이선스 메타 명시 갱신

## 6. References

- 사용자 결정 메시지 (Conversation 2026-05-16): "라이센스는 Copyleft 3.0으로 처리", "AGPL-3.0-only (권장)" 선택
- GNU AGPL v3.0 full text: https://www.gnu.org/licenses/agpl-3.0.html
- SPDX identifier: https://spdx.org/licenses/AGPL-3.0-only.html
- Apache 2.0 → GPL 호환성 (FSF): https://www.gnu.org/licenses/license-list.html#apache2
- ADR-001 (QLoRA/RL 폐기) — 사용자 데이터 보호 강화 맥락
- handoff_session_2026-05-16_qlora_deprecate_memory_qmd_plan.md — 5라운드 결정 totals
