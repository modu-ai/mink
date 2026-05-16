// global-teardown.ts — kills the mink install server after all tests complete.
// SPEC: SPEC-MINK-ONBOARDING-001 Phase 4 (AC-OB-016)
import * as fs from "fs";

export default async function globalTeardown(): Promise<void> {
  const proc = global.__MINK_SERVER_PROC__;
  if (proc && !proc.killed) {
    proc.kill("SIGTERM");
    console.log("[global-teardown] mink server stopped");
  }

  const tmpDir = global.__MINK_TEMP_DIR__;
  if (tmpDir) {
    try {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    } catch {
      // Best-effort cleanup — do not fail teardown on removal errors.
    }
  }
}
