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

describe("Doc pages", () => {
  test("index.html includes sidebar navigation", () => {
    const html = readDist("index.html");
    expect(html).toContain("Documentation");
    expect(html).toContain("🏠 Home");
  });

  test("README page title is Home", () => {
    const html = readDist("index.html");
    expect(html).toContain("<title>Home | Skill Evaluator Documentation</title>");
  });

  test("mobile menu toggle exists in built output", () => {
    const html = readDist("index.html");
    expect(html).toContain('id="menu-toggle"');
    expect(html).toContain('id="mobile-menu"');
  });

  test("ADRs are rendered", () => {
    const html = readDist("adr/0001-shell-out-to-agent-runtimes/index.html");
    expect(html).toContain("Shell out to agent runtimes");
  });

  test("GitHub link is present", () => {
    const html = readDist("index.html");
    expect(html).toContain("github.com/matt-riley/skill-evaluator");
    expect(html).toContain("🐙 GitHub");
  });
});
