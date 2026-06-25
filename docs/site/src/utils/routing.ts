export function cleanPath(key: string): string {
  return key
    .replace(/^(\.\.\/)+/, "")
    .replace(/^docs\//, "")
    .replace(/\.md$/i, "");
}

export function buildNavLinks(keys: string[], currentSlug: string) {
  return keys
    .map((key) => {
      const path = cleanPath(key);

      const segments = path.split("/");
      const name = segments[segments.length - 1];
      const title = name
        .split("-")
        .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
        .join(" ");

      return {
        path: path.toLowerCase(),
        title: title,
        isActive: path.toLowerCase() === currentSlug.toLowerCase(),
        isAdr: path.startsWith("adr/"),
      };
    })
    .sort((a, b) => {
      if (a.isAdr && !b.isAdr) return 1;
      if (!a.isAdr && b.isAdr) return -1;
      return a.title.localeCompare(b.title);
    });
}

/** File names (lowercase) excluded from the documentation nav — agent instruction files. */
export const EXCLUDED_DOCS = ["agents.md", "claude.md", "context.md"];
