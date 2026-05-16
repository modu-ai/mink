# MINK Install Wizard — Web UI

MINK 온보딩 7단계 설치 마법사의 React/Vite 프론트엔드입니다.
SPEC: SPEC-MINK-ONBOARDING-001 Phase 3A.

---

## 개발 환경 시작

### 1. 의존성 설치

```bash
cd web/install
npm install
```

### 2. 개발 서버 실행

백엔드 (`mink init --web`) 를 먼저 실행하세요:

```bash
# 터미널 1: 백엔드
MINK_DEV=1 mink init --web

# 터미널 2: 프론트엔드 개발 서버
cd web/install
npm run dev
```

브라우저에서 `http://localhost:5173/install/` 로 접속하면 됩니다.

Vite 개발 서버는 `/install/api/*` 요청을 `http://127.0.0.1:8080` 으로 프록시합니다.

---

## 프로덕션 빌드

```bash
# 1. 프론트엔드 빌드
cd web/install
npm run build

# 2. Go embed 디렉토리에 복사 (expert-backend 담당 경로로)
cp -r dist/* ../../internal/server/install/dist/

# 3. Go 전체 빌드
cd ../..
go build ./cmd/mink
```

> **주의**: `web/install/dist/` 와 `internal/server/install/dist/` 는 별개 디렉토리입니다.
> Go embed(`//go:embed all:dist`)는 `internal/server/install/dist/` 를 참조합니다.
> 빌드 후 수동으로 복사해야 합니다. Phase 3B에서 Makefile 타겟으로 자동화 예정입니다.

---

## MINK_DEV=1 모드

백엔드 서버가 `MINK_DEV=1` 환경 변수와 함께 실행될 때:

- CORS 헤더가 `localhost:5173` 에 대해 허용됩니다.
- 세션 쿠키의 `Secure` 플래그가 비활성화됩니다 (HTTP 로컬 개발 허용).
- API 요청 로깅이 상세 모드로 전환됩니다.

---

## 프록시 구성

`vite.config.ts` 에서 프록시를 설정합니다:

```
/install/api/* → http://127.0.0.1:8080/install/api/*
```

백엔드 포트를 변경하려면 `vite.config.ts` 의 `server.proxy.target` 을 수정하세요.

---

## TypeScript 타입 검사

```bash
cd web/install
npx tsc --noEmit
```

---

## 파일 구조

```
web/install/
├── src/
│   ├── main.tsx              # 앱 진입점
│   ├── App.tsx               # 루트 컴포넌트 + 스텝 라우터
│   ├── index.css             # Tailwind directives + shadcn CSS 변수
│   ├── types/
│   │   └── onboarding.ts     # Go 구조체 TypeScript 미러
│   ├── lib/
│   │   ├── api.ts            # 타입 지정 fetch 클라이언트 (CSRF double-submit)
│   │   └── utils.ts          # shadcn cn 헬퍼
│   ├── hooks/
│   │   └── useOnboarding.ts  # 세션 상태 훅
│   └── components/
│       ├── StepProgress.tsx   # 상단 진행 표시줄
│       ├── ui/
│       │   ├── button.tsx
│       │   ├── card.tsx
│       │   └── progress.tsx
│       └── steps/
│           ├── Step1Locale.tsx     # 완전 구현 (Phase 3A)
│           └── StepPlaceholder.tsx # Steps 2-7 쉘
├── dist/                     # npm run build 출력 (gitkeep 포함)
├── index.html
├── vite.config.ts
├── tailwind.config.js
├── tsconfig.json
└── package.json
```

---

## Phase 3B — Step 2~7 완성

Phase 3B에서 StepPlaceholder 스텁을 6개의 완전 구현 컴포넌트로 교체했습니다:

- **Step2Model** — Ollama 감지 상태 표시 + 모델 선택 Input
- **Step3CLI** — claude/gemini/codex 도구 감지 및 Checkbox 선택
- **Step4Persona** — 이름/경어 수준(RadioGroup)/대명사/소울 마크다운 (이름 필수 검증 포함)
- **Step5Provider** — 공급자 Select + 인증 방법 RadioGroup + API 키 password Input (show/hide)
- **Step6Messenger** — 채널 유형 Select + 조건부 토큰/Webhook URL Input
- **Step7Consent** — 4개 Checkbox + GDPR 지역 명시적 동의 RadioGroup (GDPR Skip 차단)

추가된 shadcn/ui 프리미티브: `input.tsx`, `label.tsx`, `checkbox.tsx`, `radio-group.tsx`, `select.tsx`, `textarea.tsx`

MINK 브랜드 테마 (`index.css`):
- Primary: `#6B5BFF` (hsl 247 100% 68%)
- Accent: `#FFB347` (hsl 31 100% 64%)
- Destructive: `#FF5C7C` (hsl 348 100% 68%)
- Success: `#4ADE80` (hsl 142 71% 58%)

---

## 문제 해결

**`npm run dev` 후 API 502**:
백엔드(`mink init --web`)가 실행 중인지 확인하세요. 포트 8080에서 수신 대기해야 합니다.

**TypeScript 오류 `Cannot find module '@/*'`**:
`tsconfig.app.json` 의 `paths` 설정과 `vite.config.ts` 의 `resolve.alias` 가 일치하는지 확인하세요.

**빌드 후 Go embed 실패**:
`internal/server/install/dist/` 디렉토리에 `dist/.gitkeep` 가 있는지 확인하세요. `cp -r dist/* ...` 로 복사 후 빌드하세요.
