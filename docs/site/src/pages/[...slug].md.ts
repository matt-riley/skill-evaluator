import type { APIRoute } from "astro";
import { renderMarkdownAlternate } from "@jdevalk/astro-seo-graph";
import { cleanPath, EXCLUDED_DOCS } from "../utils/routing";

const rootDocs = import.meta.glob("../../../../*.md", { eager: true });
const adrDocs = import.meta.glob("../../../../docs/adr/*.md", { eager: true });
const guideDocs = import.meta.glob("../../../../docs/guides/*.md", { eager: true });

const allFiles: Record<string, any> = Object.fromEntries(
  Object.entries({ ...rootDocs, ...adrDocs, ...guideDocs }).filter(
    ([key]) => !EXCLUDED_DOCS.some((doc) => key.toLowerCase().endsWith(doc)),
  ),
);

export function getStaticPaths() {
  return Object.keys(allFiles).map((key) => {
    const slug = cleanPath(key).toLowerCase();
    return { params: { slug: slug === "readme" ? "index" : slug } };
  });
}

export const GET: APIRoute = ({ params }) => {
  const currentSlug = (params.slug || "README").toLowerCase();
  const lookupSlug = currentSlug === "index" ? "readme" : currentSlug;
  const entry = Object.entries(allFiles).find(
    ([key]) => cleanPath(key).toLowerCase() === lookupSlug,
  );
  if (!entry) {
    return new Response("Not found", { status: 404 });
  }

  const [, mod] = entry;
  const body = typeof mod.rawContent === "function" ? mod.rawContent() : "";
  const title = mod.frontmatter?.title || (lookupSlug === "readme" ? "Home" : lookupSlug);
  const description = mod.frontmatter?.description || "";
  const canonicalPath = lookupSlug === "readme" ? "" : `${lookupSlug}/`;

  const { markdown, tokenCount, canonicalHref } = renderMarkdownAlternate({
    frontmatter: {
      title,
      canonical: `https://skilleval.mattriley.tools/${canonicalPath}`,
      description,
    },
    body,
  });

  return new Response(markdown, {
    headers: {
      "Content-Type": "text/markdown; charset=utf-8",
      "Cache-Control": "max-age=300",
      "X-Robots-Tag": "noindex, follow",
      "X-Markdown-Tokens": String(tokenCount),
      Link: `<${canonicalHref}>; rel="canonical"`,
    },
  });
};
