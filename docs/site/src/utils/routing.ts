export function cleanPath(key: string): string {
  return key
    .replace(/^(\.\.\/)+/, "")
    .replace(/^docs\//, "")
    .replace(/\.md$/i, "");
}

type NavMeta = { path: string; title?: string };

const ORDERED_PAGES: NavMeta[] = [
  { path: "index", title: "Home" },
  { path: "quick-start" },
  { path: "eval-workflow" },
  { path: "commands" },
  { path: "configuration" },
  { path: "workspace" },
  { path: "guides/first-eval" },
  { path: "guides/writing-evals" },
  { path: "guides/reading-results" },
  { path: "guides/giving-feedback" },
  { path: "guides/auto-fixing" },
  { path: "guides/cross-model" },
];

function formatTitle(name: string): string {
  return name.toLowerCase().replace(/-/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

export function buildNavLinks(keys: string[]) {
  const orderMap = new Map(ORDERED_PAGES.map((m, i) => [m.path, i]));

  const links = keys.map((key) => {
    const path = cleanPath(key).toLowerCase();
    const name = path.split("/").pop()!;
    const meta = ORDERED_PAGES.find((m) => m.path === path);

    return {
      path,
      title: meta?.title ?? formatTitle(name),
      isAdr: path.startsWith("adr/"),
      isGuide: path.startsWith("guides/"),
    };
  });

  return links.sort((a, b) => {
    if (a.path === "changelog") return 1;
    if (b.path === "changelog") return -1;

    if (a.isAdr && !b.isAdr) return 1;
    if (!a.isAdr && b.isAdr) return -1;

    if (a.isGuide && !b.isGuide) return 1;
    if (!a.isGuide && b.isGuide) return -1;

    const ai = orderMap.get(a.path) ?? Infinity;
    const bi = orderMap.get(b.path) ?? Infinity;
    if (ai !== Infinity || bi !== Infinity) return ai - bi;

    return a.title.localeCompare(b.title);
  });
}

export function extractDescription(raw: string): string {
  const paragraph = raw
    .replace(/<!--[\s\S]*?-->/g, "")
    .split(/\n+/)
    .map((line) => line.trim())
    .find((line) => line.length > 0 && !line.startsWith("#"));
  if (!paragraph) return "";

  const plain = paragraph
    .replace(/!?\[([^\]]*)\]\([^)]*\)/g, "$1")
    .replace(/[_*`]/g, "")
    .replace(/<[^>]+>/g, "");
  return plain.length > 160 ? `${plain.slice(0, 157)}...` : plain;
}

export const GITHUB_URL = "https://github.com/matt-riley/skill-evaluator";

/** File names (lowercase) excluded from the documentation nav — agent instruction files. */
export const EXCLUDED_DOCS = ["agents.md", "claude.md", "context.md", "readme.md"];
