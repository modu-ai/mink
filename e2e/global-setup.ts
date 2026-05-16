// global-setup.ts — spawns the mink install server before Playwright tests run.
// Writes MINK_WEB_BASE_URL to process.env so tests can connect.
// SPEC: SPEC-MINK-ONBOARDING-001 Phase 4 (AC-OB-016)
import { spawn, ChildProcess } from "child_process";
import * as fs from "fs";
import * as os from "os";
import * as path from "path";

declare global {
  // eslint-disable-next-line no-var
  var __MINK_SERVER_PROC__: ChildProcess | undefined;
  // eslint-disable-next-line no-var
  var __MINK_TEMP_DIR__: string | undefined;
}

// Resolve path to the mink binary.
function resolveMinkBin(): string {
  const envBin = process.env.MINK_BIN;
  if (envBin) return envBin;
  // Fallback: look for mink in the repo root (built by CI before this runs).
  const repoRoot = path.resolve(__dirname, "..");
  const local = path.join(repoRoot, "mink");
  if (fs.existsSync(local)) return local;
  throw new Error(
    "mink binary not found. Set MINK_BIN env var or build with: go build -o mink ./cmd/mink"
  );
}

export default async function globalSetup(): Promise<void> {
  const minkBin = resolveMinkBin();

  // Create isolated temp dirs so the server does not write to the real home.
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "mink-e2e-"));
  global.__MINK_TEMP_DIR__ = tmpDir;

  const minkHome = path.join(tmpDir, "home");
  const minkProject = path.join(tmpDir, "project");
  fs.mkdirSync(minkHome, { recursive: true });
  fs.mkdirSync(minkProject, { recursive: true });

  return new Promise((resolve, reject) => {
    const proc = spawn(minkBin, ["init", "--web"], {
      env: {
        ...process.env,
        MINK_HOME: minkHome,
        MINK_PROJECT_DIR: minkProject,
      },
      stdio: ["ignore", "pipe", "pipe"],
    });

    global.__MINK_SERVER_PROC__ = proc;

    let resolved = false;
    const timeoutHandle = setTimeout(() => {
      if (!resolved) {
        proc.kill();
        reject(
          new Error("mink init --web did not print a URL within 15 seconds")
        );
      }
    }, 15_000);

    // Parse the URL from stdout: "MINK install wizard ready at http://127.0.0.1:PORT/install"
    proc.stdout?.on("data", (chunk: Buffer) => {
      const text = chunk.toString();
      const match = text.match(/http:\/\/127\.0\.0\.1:(\d+)/);
      if (match && !resolved) {
        resolved = true;
        clearTimeout(timeoutHandle);
        const baseURL = `http://127.0.0.1:${match[1]}`;
        process.env.MINK_WEB_BASE_URL = baseURL;
        console.log(`[global-setup] mink server ready at ${baseURL}`);
        resolve();
      }
    });

    proc.stderr?.on("data", (chunk: Buffer) => {
      // Informational: log server stderr but don't fail unless process dies.
      process.stderr.write(`[mink] ${chunk}`);
    });

    proc.on("error", (err) => {
      if (!resolved) {
        resolved = true;
        clearTimeout(timeoutHandle);
        reject(new Error(`mink process error: ${err.message}`));
      }
    });

    proc.on("exit", (code) => {
      if (!resolved && code !== null && code !== 0) {
        resolved = true;
        clearTimeout(timeoutHandle);
        reject(new Error(`mink exited with code ${code} before URL was printed`));
      }
    });
  });
}
