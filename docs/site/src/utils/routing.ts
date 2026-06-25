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

  return lowerName
    .split("-")
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(" ");
}

function isChangelog(link: { path: string }): boolean {
  return link.path === "changelog";
}

export function buildNavLinks(keys: string[], currentSlug: string) {
  const links = keys.map((key) => {
    const path = cleanPath(key);
    const segments = path.split("/");
    const name = segments[segments.length - 1];

    return {
      path: path.toLowerCase(),
      title: formatTitle(name),
      isActive: path.toLowerCase() === currentSlug.toLowerCase(),
      isAdr: path.startsWith("adr/"),
    };
  });

  return links.sort((a, b) => {
    const aIsChangelog = isChangelog(a);
    const bIsChangelog = isChangelog(b);

    // Changelog is always last, even below ADRs.
    if (aIsChangelog || bIsChangelog) {
      return aIsChangelog ? 1 : -1;
    }

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

export const GITHUB_URL = "https://github.com/matt-riley/skill-evaluator";

/** File names (lowercase) excluded from the documentation nav — agent instruction files. */
export const EXCLUDED_DOCS = ["agents.md", "claude.md", "context.md"];
