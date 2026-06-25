// @ts-check
import { defineConfig } from "astro/config";
import cloudflare from "@astrojs/cloudflare";
import tailwindcss from "@tailwindcss/vite";
import sitemap from "@astrojs/sitemap";
import seoGraph from "@jdevalk/astro-seo-graph/integration";

// https://astro.build/config
export default defineConfig({
  site: "https://skilleval.mattriley.tools",
  output: "static",
  adapter: cloudflare({ imageService: "compile" }),
  integrations: [
    sitemap(),
    seoGraph({
      validateH1: true,
      validateUniqueMetadata: true,
      validateImageAlt: true,
      validateMetadataLength: true,
      validateInternalLinks: true,
      llmsTxt: {
        title: "Skill Eval Documentation",
        siteUrl: "https://skilleval.mattriley.tools",
      },
      markdownAlternate: true,
    }),
  ],
  vite: {
    plugins: [tailwindcss()],
    server: {
      fs: {
        allow: ["../.."],
      },
    },
  },
});
