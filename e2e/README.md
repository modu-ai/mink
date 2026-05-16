# MINK Install Wizard E2E Speedrun Tests

Playwright-based end-to-end speedrun tests for the MINK install wizard (AC-OB-016).

## SLA targets

| Path | Tool | SLA |
|------|------|-----|
| Web UI (Step 1 → Step 7 → completion) | Playwright + Chromium | ≤ 4 minutes |
| CLI (`mink init --yes`) | bash harness | ≤ 3 minutes |

## Prerequisites

- Go 1.26+
- Node.js 22+ with npm
- Chromium (installed by Playwright on first run)
- Web bundle built and copied to `internal/server/install/dist/`

## Running locally

### 1. Build the web bundle

```bash
cd web/install
npm ci
npm run build
mkdir -p ../../internal/server/install/dist
cp -r dist/* ../../internal/server/install/dist/
cd ../..
```

### 2. Build the mink binary

```bash
go build -o /tmp/mink ./cmd/mink
```

### 3. Run the Web UI Playwright speedrun

```bash
cd e2e
npm ci
npx playwright install chromium --with-deps
MINK_BIN=/tmp/mink npx playwright test
```

### 4. Run the CLI speedrun

```bash
MINK_BIN=/tmp/mink bash scripts/cli-install-speedrun.sh
```

## How it works

### Web UI speedrun (`install-wizard-speedrun.spec.ts`)

1. `global-setup.ts` spawns `mink init --web` and parses the URL from stdout.
2. The Playwright test navigates to `/install` and drives through all 7 steps:
   - Step 1: Selects the default KR preset and clicks Continue.
   - Steps 2, 3, 5, 6: Clicks Skip.
   - Step 4: Fills "SpeedrunTester" as the persona name and clicks Continue.
   - Step 7: Clicks Continue (KR / PIPA region — no GDPR block).
3. Waits for the "온보딩 완료 / Onboarding Complete" completion screen.
4. Asserts elapsed time ≤ 240 s.
5. `global-teardown.ts` kills the mink server and removes temp dirs.

### CLI speedrun (`scripts/cli-install-speedrun.sh`)

Builds (or reuses `$MINK_BIN`), runs `mink init --yes --dry-run`, and asserts
elapsed time ≤ 180 s. No disk writes occur in dry-run mode.

## Configuration

| Env var | Default | Description |
|---------|---------|-------------|
| `MINK_BIN` | auto-built | Path to a pre-built mink binary |
| `SPEEDRUN_SLA_SECONDS` | `180` | CLI SLA in seconds |
| `MINK_WEB_BASE_URL` | set by global-setup | Base URL for Playwright |

## CI integration

The `.github/workflows/install-wizard-e2e.yml` workflow runs both jobs on:
- Every PR touching onboarding code, `web/install/`, or `e2e/`
- Every push to `main`
