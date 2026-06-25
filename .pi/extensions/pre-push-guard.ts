/**
 * Pre-Push Guard Extension
 *
 * Blocks git push/commit until format and lint checks pass.
 * Primary: runs mise fmt + mise lint if mise.toml exists.
 * Fallback: auto-detects formatters/linters from project files.
 */

import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";

type Check = { label: string; command: string; args: string[] };

async function detectChecks(cwd: string, pi: ExtensionAPI): Promise<Check[]> {
  // Primary: mise.toml tasks
  if (existsSync(join(cwd, "mise.toml"))) {
    const checks: Check[] = [];
    try {
      const { stdout } = await pi.exec("mise", ["ls"]);
      if (stdout.includes("fmt")) checks.push({ label: "fmt", command: "mise", args: ["run", "fmt"] });
      if (stdout.includes("lint")) checks.push({ label: "lint", command: "mise", args: ["run", "lint"] });
    } catch {
      // mise not installed or failed — fall through to auto-detect
    }
    if (checks.length > 0) return checks;
  }

  // Fallback: auto-detect from project files
  const checks: Check[] = [];

  // Go
  if (existsSync(join(cwd, "go.mod"))) {
    checks.push({ label: "gofmt", command: "gofmt", args: ["-d", "."] });
    // golangci-lint: check if installed or configured
    if (existsSync(join(cwd, ".golangci.yml")) || existsSync(join(cwd, ".golangci.yaml"))) {
      checks.push({ label: "golangci-lint", command: "golangci-lint", args: ["run", "./..."] });
    }
  }

  // JavaScript/TypeScript
  if (existsSync(join(cwd, "package.json"))) {
    try {
      const pkg = JSON.parse(readFileSync(join(cwd, "package.json"), "utf-8"));
      const scripts = pkg.scripts || {};
      if (scripts.format) {
        checks.push({ label: "format", command: "npm", args: ["run", "format"] });
      } else if (scripts.fmt) {
        checks.push({ label: "fmt", command: "npm", args: ["run", "fmt"] });
      }
      if (scripts.lint) {
        checks.push({ label: "lint", command: "npm", args: ["run", "lint"] });
      }
    } catch {
      // package.json unreadable — skip JS checks
    }
  }

  // Python
  if (existsSync(join(cwd, "pyproject.toml")) || existsSync(join(cwd, "setup.cfg"))) {
    checks.push({ label: "ruff", command: "ruff", args: ["check", "."] });
  }

  return checks;
}

function isPushOrCommit(command: string): boolean {
  return /git\s+(push|commit)/.test(command);
}

export default function (pi: ExtensionAPI) {
  pi.on("tool_call", async (event, ctx) => {
    if (event.toolName !== "bash") return;
    const cmd = (event.input as { command?: string }).command ?? "";
    if (!isPushOrCommit(cmd)) return;

    const checks = await detectChecks(ctx.cwd, pi);
    if (checks.length === 0) return;

    // Show running widget
    const widgetId = "pre-push-guard";
    const statusLines = ["🔍 Pre-push checks"];
    for (const c of checks) statusLines.push(`  ⏳ ${c.label}...`);
    ctx.ui.setWidget(widgetId, statusLines);

    const failures: string[] = [];
    for (let i = 0; i < checks.length; i++) {
      const check = checks[i];
      const { code, stderr, stdout } = await pi.exec(check.command, check.args);
      if (code !== 0) {
        const output = (stderr || stdout).slice(0, 500);
        failures.push(`${check.label} failed (exit ${code})\n${output}`);
        statusLines[i + 1] = `  ✗ ${check.label} (exit ${code})`;
      } else {
        statusLines[i + 1] = `  ✓ ${check.label}`;
      }
      ctx.ui.setWidget(widgetId, statusLines);
    }

    // Show final result briefly, then clear
    const allPassed = failures.length === 0;
    statusLines.push(allPassed ? "  ✅ All checks passed" : "  ❌ Checks failed");
    ctx.ui.setWidget(widgetId, statusLines);

    setTimeout(() => ctx.ui.setWidget(widgetId, []), 3000);

    if (allPassed) return;

    const reason = "Pre-push checks failed:\n\n" + failures.join("\n\n");
    if (!ctx.hasUI) return { block: true, reason };

    ctx.ui.notify("Format/lint checks failed", "error");
    const choice = await ctx.ui.select(
      `${failures.length} check(s) failed. Push anyway?`,
      ["No — fix first (block)", "Yes — I know what I'm doing"],
    );

    if (choice !== "Yes — I know what I'm doing") {
      return { block: true, reason };
    }
  });
}
