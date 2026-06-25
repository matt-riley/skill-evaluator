export function cleanPath(key: string): string {
  return key
    .replace(/^(\.\.\/)+/, "")
    .replace(/^docs\//, "")
    .replace(/\.md$/i, "");
}

const DOC_ORDER = [
  "readme",
  "quick-start",
  "eval-workflow",
  "commands",
  "configuration",
  "workspace",
];
const TITLE_OVERRIDES: Record<string, string> = {
  readme: "Home",
};

function formatTitle(name: string): string {
  const lowerName = name.toLowerCase();
  if (TITLE_OVERRIDES[lowerName]) return TITLE_OVERRIDES[lowerName];

  return lowerName.replace(/-/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

export function buildNavLinks(keys: string[]) {
  const links = keys.map((key) => {
    const path = cleanPath(key);
    const segments = path.split("/");
    const name = segments[segments.length - 1];

    return {
      path: path.toLowerCase(),
      title: formatTitle(name),
      isAdr: path.startsWith("adr/"),
    };
  });

  return links.sort((a, b) => {
    // Changelog is always last, even below ADRs.
    if (a.path === "changelog") return 1;
    if (b.path === "changelog") return -1;

    if (a.isAdr && !b.isAdr) return 1;
    if (!a.isAdr && b.isAdr) return -1;

    const aIndex = DOC_ORDER.indexOf(a.path);
    const bIndex = DOC_ORDER.indexOf(b.path);
    if (aIndex >= 0 && bIndex >= 0) return aIndex - bIndex;
    if (aIndex >= 0) return -1;
    if (bIndex >= 0) return 1;

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
export const EXCLUDED_DOCS = ["agents.md", "claude.md", "context.md"];
