import { describe, expect, it } from "vitest";
import { currentVSCodeSummary, vscodeIsReady } from "./helpers";
import { makeVSCodeDetect } from "../../test/fixtures";

describe("shared vscode helpers", () => {
  it("treats legacy settings residue as not ready even if managed shim is installed", () => {
    const detect = makeVSCodeDetect({
      settings: {
        path: "/tmp/settings.json",
        exists: true,
        cliExecutable: "/usr/local/bin/codex-remote",
        matchesBinary: true,
      },
      latestShim: {
        entrypoint: "/tmp/codex-shim.js",
        exists: true,
        realBinaryPath: "/usr/local/bin/codex",
        realBinaryExists: true,
        installed: true,
        matchesBinary: true,
      },
      needsShimReinstall: false,
    });

    expect(vscodeIsReady(detect)).toBe(false);
    expect(currentVSCodeSummary(detect)).toBe("检测到旧版 settings.json 接入，需迁移到扩展入口");
  });
});
