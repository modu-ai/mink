# Migration Log — SPEC-GOOSE-BRAND-RENAME-001 (AI.GOOSE 브랜드 통일)

작성일: 2026-04-26
SPEC: SPEC-GOOSE-BRAND-RENAME-001 v0.1.1
담당: expert-refactoring

---

## Baseline (Phase 1 캡처)

AC-BR-007 / AC-BR-008 / AC-BR-009 / AC-BR-011 검증의 기준점.

### go list -m 출력

```
github.com/modu-ai/goose
```

### Go package 선언 카운트 (`grep -rh "^package " --include="*.go"`)

총 85개 패키지 선언:

```
  16 package skill
  11 package hook
   9 package tools
   9 package context
   9 package anthropic_test
   9 package anthropic
   8 package credential
   8 package core
   7 package provider
   7 package errorclass
   7 package context_test
   7 package config
   6 package router
   6 package message
   6 package loop_test
   6 package loop
   5 package provider_test
   5 package openai
   5 package message_test
   5 package grpc
   5 package file
   5 package credential_test
   4 package tools_test
   4 package router_test
   4 package permissions
   4 package main
   3 package query_test
   3 package query
   3 package kimi
   3 package core_test
   2 package permission
   2 package hook_test
   2 package goosev1
   2 package google
   2 package glm_test
   2 package glm
   2 package factory_test
   2 package factory
   1 package xai_test
   1 package xai
   1 package tool_test
   1 package tool
   1 package together_test
   1 package together
   1 package testsupport
   1 package testhelper
   1 package terminal_test
   1 package terminal
   1 package search_test
   1 package search
   1 package ratelimit_test
   1 package ratelimit
   1 package qwen_test
   1 package qwen
   1 package permissions_test
   1 package permission_test
   1 package openrouter_test
   1 package openrouter
   1 package openai_test
   1 package ollama_test
   1 package ollama
   1 package naming_test
   1 package naming
   1 package mistral_test
   1 package mistral
   1 package mcp
   1 package kimi_test
   1 package health
   1 package grpc_test
   1 package groq_test
   1 package groq
   1 package google_test
   1 package fireworks_test
   1 package fireworks
   1 package file_test
   1 package errorclass_test
   1 package deepseek_test
   1 package deepseek
   1 package config_test
   1 package cerebras_test
   1 package cerebras
   1 package cache_test
   1 package cache
   1 package builtin_test
   1 package builtin
```

### `type Goose*` 카운트

```
0
```

### `ls cmd/` 출력

```
goosed
```

### SPEC 디렉토리 목록 (`ls .moai/specs/ | grep "^SPEC-"` 해당 항목)

총 64개 SPEC 디렉토리:

```
SPEC-AGENCY-ABSORB-001
SPEC-AGENCY-CLEANUP-002
SPEC-GOOSE-A2A-001
SPEC-GOOSE-ADAPTER-001
SPEC-GOOSE-ADAPTER-002
SPEC-GOOSE-AGENT-001
SPEC-GOOSE-ARCH-REDESIGN-v0.2
SPEC-GOOSE-AUDIT-001
SPEC-GOOSE-AUTH-001
SPEC-GOOSE-BRAND-RENAME-001
SPEC-GOOSE-BRIDGE-001
SPEC-GOOSE-BRIEFING-001
SPEC-GOOSE-CALENDAR-001
SPEC-GOOSE-CLI-001
SPEC-GOOSE-COMMAND-001
SPEC-GOOSE-COMPRESSOR-001
SPEC-GOOSE-CONFIG-001
SPEC-GOOSE-CONTEXT-001
SPEC-GOOSE-CORE-001
SPEC-GOOSE-CREDENTIAL-PROXY-001
SPEC-GOOSE-CREDPOOL-001
SPEC-GOOSE-DAEMON-WIRE-001
SPEC-GOOSE-DESKTOP-001
SPEC-GOOSE-ERROR-CLASS-001
SPEC-GOOSE-FORTUNE-001
SPEC-GOOSE-FS-ACCESS-001
SPEC-GOOSE-GATEWAY-001
SPEC-GOOSE-GATEWAY-TG-001
SPEC-GOOSE-HEALTH-001
SPEC-GOOSE-HOOK-001
SPEC-GOOSE-I18N-001
SPEC-GOOSE-IDENTITY-001
SPEC-GOOSE-INSIGHTS-001
SPEC-GOOSE-JOURNAL-001
SPEC-GOOSE-LLM-001
SPEC-GOOSE-LOCALE-001
SPEC-GOOSE-LORA-001
SPEC-GOOSE-MCP-001
SPEC-GOOSE-MEMORY-001
SPEC-GOOSE-NOTIFY-001
SPEC-GOOSE-ONBOARDING-001
SPEC-GOOSE-PAI-CONTEXT-001
SPEC-GOOSE-PERMISSION-001
SPEC-GOOSE-PLUGIN-001
SPEC-GOOSE-PROMPT-CACHE-001
SPEC-GOOSE-QMD-001
SPEC-GOOSE-QUERY-001
SPEC-GOOSE-RATELIMIT-001
SPEC-GOOSE-REFLECT-001
SPEC-GOOSE-REGION-SKILLS-001
SPEC-GOOSE-RELAY-001
SPEC-GOOSE-RITUAL-001
SPEC-GOOSE-ROLLBACK-001
SPEC-GOOSE-ROUTER-001
SPEC-GOOSE-SAFETY-001
SPEC-GOOSE-SCHEDULER-001
SPEC-GOOSE-SECURITY-SANDBOX-001
SPEC-GOOSE-SELF-CRITIQUE-001
SPEC-GOOSE-SIGNING-001
SPEC-GOOSE-SKILLS-001
SPEC-GOOSE-SUBAGENT-001
SPEC-GOOSE-TOOLS-001
SPEC-GOOSE-TRAJECTORY-001
SPEC-GOOSE-TRANSPORT-001
SPEC-GOOSE-VECTOR-001
SPEC-GOOSE-WEATHER-001
SPEC-GOOSE-WEBUI-001
```

### git log --oneline snapshot (최근 30건)

```
147ea27 feat(prompt-cache): SPEC-GOOSE-PROMPT-CACHE-001 v0.1.0 — Anthropic system_and_3 BreakpointPlanner
055c40e feat(error-class): SPEC-GOOSE-ERROR-CLASS-001 v0.1.1 — Error Classifier (14 FailoverReason)
6c140aa feat(daemon-wire): SPEC-GOOSE-DAEMON-WIRE-001 v0.1.0 — goosed production daemon wire-up
bfaeca7 docs(spec): SPEC-GOOSE-BRAND-RENAME-001 v0.1.1 초안 — AI.GOOSE 브랜드 통일 (#25)
dcbb200 feat(transport): SPEC-GOOSE-TRANSPORT-001 v0.1.2 — gRPC 서버 + proto 스키마 기본 계약 (#24)
bb58801 docs(spec): SPEC-GOOSE-DAEMON-WIRE-001 v0.1.0 초안 작성 — goosed daemon wire-up plan (#23)
d1bb9bc feat(skill): SPEC-GOOSE-SKILLS-001 v0.3.0 — Progressive Disclosure Skill System 구현 (M2) (#22)
ad381a1 docs(spec): SPEC-GOOSE-CONFIG-001 v0.3.0 → v0.3.1 implementation closure 반영 (#21)
da8d7f1 feat(config): SPEC-GOOSE-CONFIG-001 v0.3.0 — 계층형 Config 로더 구현 (#20)
f5c92c7 docs(spec): SPEC-GOOSE-QUERY-001 status sync — planned → implemented (#19)
f2d00e4 docs(spec): batch status sync — HOOK-001/TOOLS-001/CONTEXT-001 planned → implemented (#18)
cd9782c docs(changelog): SPEC-GOOSE-CORE-001 v1.1.0 sync — Unreleased에 SessionRegistry/DrainConsumer 항목 추가 (#17)
77b79cd feat(core): SPEC-GOOSE-CORE-001 v1.1.0 — SessionRegistry + DrainConsumer 구현 (#16)
227faa8 docs(spec): SPEC-GOOSE-CORE-001 v1.0.0 → v1.1.0 — cross-package interface contract amendment (#15)
b16f9e2 docs(audit): cross-package interface stub audit (TOOLS-001/HOOK-001 → 4 consumer SPECs) (#14)
68fefc3 docs(spec): SPEC-GOOSE-ADAPTER-002 v1.0.0 → v1.1.0 — OI-1/2/3 closure 반영 (#13)
011ff07 feat(llm/provider): SPEC-GOOSE-ADAPTER-002 v0.3 — OI-1/2/3 구현 (#12)
6ac25c8 feat(hook): SPEC-GOOSE-HOOK-001 — PostSamplingHooks dispatcher 구현 (#11)
18ee5fb feat(tools): SPEC-GOOSE-TOOLS-001 — Tool Registry/Executor + Built-in 6종 구현 (#10)
b7ca5ca feat(context): SPEC-GOOSE-CONTEXT-001 — DefaultCompactor 구현 (#9)
a1dc026 chore(cleanup): SPEC-AGENCY-CLEANUP-002 — legacy agency 28 artifact 정리 (#8)
8c49ab7 feat(query): SPEC-GOOSE-QUERY-001 S5~S9 — 16/16 AC GREEN 달성 (#7)
1b1d1cd chore(moai): self-dev 메타 동기화 — output-style + config 2.14.0 + sessions
586b44c feat(moai): SPEC-GOOSE-QUERY-001 + mass audit Phase A/B/C remediation (#6)
b4559f9 chore(ci): GitHub Actions CI workflow 추가 (build / vet / gofmt / test -race)
5186fd1 chore(git): Enhanced GitHub Flow 5가지 개선 적용 (0.1.0 이전)
f38f1fa chore(git): 0.1.0 이전 gitflow 전환 (main > develop > feature/*)
ca13c07 chore(moai): self-dev 설정/skill/세션 리포트 일괄 정리
3bb1023 feat(adapter): SPEC-GOOSE-ADAPTER-002 — 9 OpenAI-compat Provider + Z.ai GLM (#4)
bea5df1 feat(adapter): SPEC-GOOSE-ADAPTER-001 — 6 LLM Provider + skeleton + CREDPOOL ext (#3)
```

---

## Phase 2 — 핵심 user-facing 문서 brand 통일

완료일: 2026-04-26

### 변경 파일 목록

| 파일 | 변경 라인 수 | 변경 내용 요약 |
|------|------------|--------------|
| `.moai/project/product.md` | 2 | 제목 + 프로젝트명 필드: `GOOSE-AGENT` → `AI.GOOSE` |
| `.moai/project/branding.md` | 1 | 제목: `GOOSE-AGENT` → `AI.GOOSE` |
| `.moai/project/adaptation.md` | 1 | 제목: `GOOSE-AGENT` → `AI.GOOSE` |
| `.moai/project/ecosystem.md` | 1 | 제목: `GOOSE-AGENT` → `AI.GOOSE` |
| `.moai/project/learning-engine.md` | 1 | 제목: `GOOSE-AGENT` → `AI.GOOSE` |
| `.moai/project/migration.md` | 1 | 제목: `GOOSE-AGENT` → `AI.GOOSE` |
| `.moai/project/structure.md` | 1 | 제목: `GOOSE-AGENT` → `AI.GOOSE` |
| `.moai/project/tech.md` | 1 | 제목: `GOOSE-AGENT` → `AI.GOOSE` |
| `.moai/project/token-economy.md` | 1 | 제목: `GOOSE-AGENT` → `AI.GOOSE` |
| `.moai/project/research/claude-core.md` | 1 | 용도 메타 주석: `GOOSE-AGENT` → `AI.GOOSE` |

**총 변경: 10 파일, 11 라인**

README.md, CHANGELOG.md, CLAUDE.md — brand 위반 없음 (정정 불요).

### before/after 샘플

```diff
- # GOOSE-AGENT - 제품 문서 v4.0 GLOBAL EDITION
+ # AI.GOOSE - 제품 문서 v4.0 GLOBAL EDITION

- - **프로젝트명:** GOOSE-AGENT (거위 에이전트)
+ - **프로젝트명:** AI.GOOSE (거위 에이전트)
```

---

## Phase 3 — .claude/ rules/agents/skills/commands brand 통일

완료일: 2026-04-26

### 검사 결과

`.claude/rules/**/*.md`, `.claude/agents/**/*.md`, `.claude/skills/**/*.md`, `.claude/commands/**/*.md` 전체 검사 결과:

**brand 위반 건수: 0건** (정정 불요)

research.md §6 Phase 3에서 추정한 4파일 6건은 도메인 용어(`SPEC-GOOSE-XXX-NNN` 참조 등) 또는 백틱 인용이었으며 실제 brand 표기 위반에 해당하지 않음.

변경 파일: 없음.

---

## Phase 4 — SPEC 본문 정정 결과

완료일: 2026-04-26

### 변경 SPEC 목록 (21 spec.md + 1 research.md + 1 spec-compact.md + 2 non-SPEC .md)

| 파일 | 변경 라인 수 | 변경 내용 요약 |
|------|------------|--------------|
| `SPEC-GOOSE-ADAPTER-001/spec.md` | 1 | `GOOSE-AGENT Phase 1의 ...` → `AI.GOOSE Phase 1의 ...` |
| `SPEC-GOOSE-AGENT-001/spec.md` | 1 | `GOOSE-AGENT 대화 루프의 ...` → `AI.GOOSE 대화 루프의 ...` |
| `SPEC-GOOSE-CLI-001/spec.md` | 1 | `GOOSE-AGENT의 **사용자 대면 CLI**` → `AI.GOOSE의 **사용자 대면 CLI**` |
| `SPEC-GOOSE-COMMAND-001/spec.md` | 1 | `GOOSE-AGENT의 **Slash Command...` → `AI.GOOSE의 **Slash Command...` |
| `SPEC-GOOSE-COMPRESSOR-001/spec.md` | 1 | `GOOSE-AGENT **자기진화 파이프라인의 Layer 2**` → `AI.GOOSE **자기진화...` |
| `SPEC-GOOSE-CORE-001/spec.md` | 1 | `GOOSE-AGENT의 모든 후속 기능` → `AI.GOOSE의 모든 후속 기능` |
| `SPEC-GOOSE-CREDPOOL-001/spec.md` | 1 | `GOOSE-AGENT Phase 1의 **Multi-LLM...` → `AI.GOOSE Phase 1의 **Multi-LLM...` |
| `SPEC-GOOSE-ERROR-CLASS-001/spec.md` | 1 | `GOOSE-AGENT **자기진화 파이프라인의 보조 레이어**` → `AI.GOOSE **자기진화...` |
| `SPEC-GOOSE-HOOK-001/spec.md` | 1 | `GOOSE-AGENT의 **24개 lifecycle hook...` → `AI.GOOSE의 **24개...` |
| `SPEC-GOOSE-INSIGHTS-001/spec.md` | 1 | `GOOSE-AGENT **자기진화 파이프라인의 Layer 3**` → `AI.GOOSE **자기진화...` |
| `SPEC-GOOSE-LLM-001/spec.md` | 1 | `GOOSE-AGENT의 모든 LLM 호출` → `AI.GOOSE의 모든 LLM 호출` |
| `SPEC-GOOSE-MCP-001/spec.md` | 1 | `GOOSE-AGENT의 **Model Context Protocol...` → `AI.GOOSE의 **MCP...` |
| `SPEC-GOOSE-MEMORY-001/spec.md` | 1 | `GOOSE-AGENT **자기진화 파이프라인의 Layer 4**` → `AI.GOOSE **자기진화...` |
| `SPEC-GOOSE-PLUGIN-001/spec.md` | 1 | `GOOSE-AGENT의 **Plugin Host**` → `AI.GOOSE의 **Plugin Host**` |
| `SPEC-GOOSE-QUERY-001/spec.md` | 1 | `GOOSE-AGENT의 **agentic 코어 런타임**` → `AI.GOOSE의 **agentic 코어 런타임**` |
| `SPEC-GOOSE-RATELIMIT-001/spec.md` | 1 | `GOOSE-AGENT Phase 1의 **provider 응답 헤더...` → `AI.GOOSE Phase 1의...` |
| `SPEC-GOOSE-ROUTER-001/spec.md` | 1 | `GOOSE-AGENT의 **라우팅 결정 레이어**` → `AI.GOOSE의 **라우팅...` |
| `SPEC-GOOSE-SKILLS-001/spec.md` | 1 | `GOOSE-AGENT의 **Skill 시스템**` → `AI.GOOSE의 **Skill 시스템**` |
| `SPEC-GOOSE-SUBAGENT-001/spec.md` | 1 | `GOOSE-AGENT의 **Sub-agent 런타임**` → `AI.GOOSE의 **Sub-agent 런타임**` |
| `SPEC-GOOSE-TOOLS-001/spec.md` | 1 | `GOOSE-AGENT의 **Tool 실행 인프라 계층**` → `AI.GOOSE의 **Tool...` |
| `SPEC-GOOSE-TRAJECTORY-001/spec.md` | 1 | `GOOSE-AGENT **자기진화 파이프라인의 Layer 1**` → `AI.GOOSE **자기진화...` |
| `SPEC-GOOSE-ADAPTER-002/research.md` | 1 | `GOOSE 프로젝트(module ...` → `AI.GOOSE(module ...` |
| `SPEC-GOOSE-QUERY-001/spec-compact.md` | 1 | `GOOSE-AGENT agentic 코어 런타임` → `AI.GOOSE agentic 코어 런타임` |
| `.moai/specs/IMPLEMENTATION-ORDER.md` | 1 | 제목: `GOOSE-AGENT 구현 순서 종합 보고서` → `AI.GOOSE 구현 순서 종합 보고서` |
| `.moai/specs/ROADMAP.md` | 1 | 제목: `GOOSE-AGENT SPEC 로드맵` → `AI.GOOSE SPEC 로드맵` |

**총 변경: 25 파일, 25 라인**

보존 확인:
- 모든 `## HISTORY` 섹션 내 항목: 변경 없음 (HISTORY 라인 포함 diff = 0)
- `SPEC-GOOSE-AGENT-001` 등 SPEC ID 참조 형태: 변경 없음
- `SPEC-DOC-REVIEW-2026-04-21.md`의 `SPEC-GOOSE-AGENT-001` 참조: SPEC ID이므로 보존

### Spot-check QA 결과 (전체 변경 25건 중 5건 무작위 추출)

| 점검 파일 | 변경 라인 | 판정 | 비고 |
|-----------|----------|------|------|
| `SPEC-GOOSE-CORE-001/spec.md` L32 | `GOOSE-AGENT의` → `AI.GOOSE의` | Pass | HISTORY 외부, brand 위치 정정 |
| `SPEC-GOOSE-ADAPTER-002/research.md` L14 | `GOOSE 프로젝트(` → `AI.GOOSE(` | Pass | HISTORY 없는 파일, brand 위치 정정 |
| `SPEC-GOOSE-QUERY-001/spec-compact.md` L17 | `GOOSE-AGENT agentic` → `AI.GOOSE agentic` | Pass | HISTORY 외부, brand 위치 정정 |
| `SPEC-GOOSE-TRAJECTORY-001/spec.md` L29 | `GOOSE-AGENT **자기진화 파이프라인의 Layer 1**` → `AI.GOOSE **...` | Pass | HISTORY 외부, brand 위치 정정 |
| `SPEC-GOOSE-ERROR-CLASS-001/spec.md` L29 | `GOOSE-AGENT **자기진화 파이프라인의 보조 레이어**` → `AI.GOOSE **...` | Pass | HISTORY 외부, brand 위치 정정 |

5/5 Pass — §7.5 알고리즘 분류(A) 정정과 100% 일치.

---

## Phase 6 — 최종 검증 결과

### AC-BR-007 — go list -m 재실행

```
(Phase 6 완료 후 기록)
```

### AC-BR-008 — Go package/type 카운트 재실행

```
(Phase 6 완료 후 기록)
```

### AC-BR-009 — SPEC 디렉토리 목록 재실행

```
(Phase 6 완료 후 기록)
```

---

Version: 0.1.0 (Phase 1 baseline 캡처)
Created: 2026-04-26
