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

const GUIDE_ORDER = [
  "guides/first-eval",
  "guides/reading-results",
  "guides/giving-feedback",
  "guides/auto-fixing",
  "guides/cross-model",
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
      isGuide: path.startsWith("guides/"),
    };
  });

  const orderedCompare = (a: string, b: string, order: string[]) => {
    const ai = order.indexOf(a);
    const bi = order.indexOf(b);
    if (ai >= 0 && bi >= 0) return ai - bi;
    if (ai >= 0) return -1;
    if (bi >= 0) return 1;
    return undefined;
  };

  return links.sort((a, b) => {
    // Changelog is always last, even below ADRs.
    if (a.path === "changelog") return 1;
    if (b.path === "changelog") return -1;

    // ADRs sink below guides and normal docs.
    if (a.isAdr && !b.isAdr) return 1;
    if (!a.isAdr && b.isAdr) return -1;

    // Guides sit between normal docs and ADRs.
    if (a.isGuide && !b.isGuide) return 1;
    if (!a.isGuide && b.isGuide) return -1;

    const docCmp = orderedCompare(a.path, b.path, DOC_ORDER);
    if (docCmp !== undefined) return docCmp;

    const guideCmp = orderedCompare(a.path, b.path, GUIDE_ORDER);
    if (guideCmp !== undefined) return guideCmp;

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
