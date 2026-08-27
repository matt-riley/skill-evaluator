import { describe, test, expect } from "vitest";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const distDir = resolve(import.meta.dirname, "../dist/client");

function readDist(filename: string): string {
  return readFileSync(resolve(distDir, filename), "utf-8");
}

describe("Layout (rendered output)", () => {
  const html = readDist("index.html");

  test("renders the page title in <title> tag", () => {
    expect(html).toMatch(/<title>[^<]+<\/title>/);
  });

  test("includes meta charset utf-8", () => {
    expect(html).toContain('<meta charset="utf-8"');
  });

  test("includes viewport meta tag", () => {
    expect(html).toContain('name="viewport"');
  });

  test("body has font-sans class", () => {
    expect(html).toContain("font-sans");
  });

  test("includes CSP meta tag", () => {
    expect(html).toContain("Content-Security-Policy");
  });
});

describe("404 page", () => {
  const html = readDist("404.html");

  test("renders 404 heading text", () => {
    expect(html).toContain("404");
  });

  test("includes a link back to home", () => {
    expect(html).toContain('href="/"');
    expect(html).toContain("Back to Home");
  });
});

describe("Home page", () => {
  const html = readDist("index.html");

  test("homepage title is Home", () => {
    expect(html).toContain("Home");
  });

  test("renders the hero headline", () => {
    expect(html).toContain("Stop guessing.");
  });

  test("renders the live terminal demo", () => {
    expect(html).toContain("skill-eval loop");
    expect(html).toContain("benchmark");
  });

  test("mobile menu toggle exists in built output", () => {
    expect(html).toContain('id="menu-toggle"');
  });

  test("GitHub link is present", () => {
    expect(html).toContain("github.com/matt-riley/skill-evaluator");
    expect(html).toContain("GitHub");
  });
});

describe("Doc pages", () => {
  const html = readDist("quick-start/index.html");

  test("doc page includes sidebar navigation", () => {
    expect(html).toContain("Documentation");
    expect(html).toContain("Guides");
  });

  test("doc page includes mobile drawer", () => {
    expect(html).toContain('id="mobile-menu"');
  });

  test("doc page renders page hero with title", () => {
    expect(html).toContain("Quick Start");
  });

  test("ADRs are rendered", () => {
    const adr = readDist("adr/0001-shell-out-to-agent-runtimes/index.html");
    expect(adr).toContain("Shell out to agent runtimes");
  });

  test("changelog is rendered at /changelog/", () => {
    const changelog = readDist("changelog/index.html");
    expect(changelog).toContain("Changelog");
  });
});
