import { describe, expect, it } from "vitest";
import { vscodeReadinessText } from "./helpers";
import { makeVSCodeDetect } from "../../test/fixtures";

describe("admin vscode helpers", () => {
  it("prioritizes migration guidance over ready text when legacy settings residue remains", () => {
    const detect = makeVSCodeDetect({
      latestBundleEntrypoint: "/tmp/codex-shim.js",
      recordedBundleEntrypoint: "/tmp/codex-shim.js",
      candidateBundleEntrypoints: ["/tmp/codex-shim.js"],
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

    expect(vscodeReadinessText(detect)).toBe("检测到旧版 settings.json 接入，建议迁移到扩展入口。");
  });
});
