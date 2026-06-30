---
title: Changelog
description: Changelog for skill-evaluator, listing releases and notable changes to the CLI, configuration, documentation site, and build tooling.
---

# Changelog

## [1.3.0](https://github.com/matt-riley/skill-evaluator/compare/v1.2.0...v1.3.0) (2026-06-30)


### Features

* context propagation — ctx through CmdBuilder, per-invocation timeout, graceful cancellation ([#14](https://github.com/matt-riley/skill-evaluator/issues/14)) ([2089e67](https://github.com/matt-riley/skill-evaluator/commit/2089e67ed425e6a959402fee1f1fdabe7e607398))
* **import-agit:** integrate with agengit steps/eval, --merge, source metadata, smart filtering ([#26](https://github.com/matt-riley/skill-evaluator/issues/26)) ([ca3bf55](https://github.com/matt-riley/skill-evaluator/commit/ca3bf5559542be6e59d48c21c90f35df68fde506))


### Bug Fixes

* collect errors in cmdRun instead of aborting; pass --model/--skill to generic agent ([#22](https://github.com/matt-riley/skill-evaluator/issues/22)) ([ab608e3](https://github.com/matt-riley/skill-evaluator/commit/ab608e34852a45a8b3612a629625fda405b06df0))
* make CmdBuilder a parameter instead of package-level var to prevent races ([#23](https://github.com/matt-riley/skill-evaluator/issues/23)) ([3f62ebe](https://github.com/matt-riley/skill-evaluator/commit/3f62ebe88d6933d3f7b66f7d9d0565da139658ec)), closes [#17](https://github.com/matt-riley/skill-evaluator/issues/17)
* remediate all 22 security vulnerabilities from audit ([c8f6628](https://github.com/matt-riley/skill-evaluator/commit/c8f662851d20a50a2761a2c1182ed4047c225747))
* remove copilot from schema enum, use [@v2](https://github.com/v2) refs in release.yml ([#21](https://github.com/matt-riley/skill-evaluator/issues/21)) ([678c160](https://github.com/matt-riley/skill-evaluator/commit/678c1603cea2ce992636b66b902db7a3ddb57ddc))

## [1.2.0](https://github.com/matt-riley/skill-evaluator/compare/v1.1.0...v1.2.0) (2026-06-27)


### Features

* import-agit converts recorded sessions into evals.json corpora ([6670222](https://github.com/matt-riley/skill-evaluator/commit/66702227c07ed2473ab63127644bb9f8be31cc6c))


### Bug Fixes

* **benchmark:** correct best/worst ranking, populate run_summary, extract pi token counts ([83a9354](https://github.com/matt-riley/skill-evaluator/commit/83a935480c142c284544048352cb01775ea4c9bb))
* **lint:** remove unused cliJSONEnvelopeKey const flagged by golangci-lint ([2fb9c82](https://github.com/matt-riley/skill-evaluator/commit/2fb9c82b1056005dfeeebfcc1be37fd27e07f265))

## [1.1.0](https://github.com/matt-riley/skill-evaluator/compare/v1.0.0...v1.1.0) (2026-06-27)


### Features

* add --baseline-only flag to run and loop commands ([94771cb](https://github.com/matt-riley/skill-evaluator/commit/94771cb7479d616df503468b3535573667d3bbd5))
* add --llm-suggestions and high-baseline eval-quality hint ([26cca28](https://github.com/matt-riley/skill-evaluator/commit/26cca28cbd42e60a9fb9e25fce1692793b413dbc))
* add HTML report command with actionable suggestions ([3ca892c](https://github.com/matt-riley/skill-evaluator/commit/3ca892c583b8e060b90e9d6ae14d8b4a76865856))


### Bug Fixes

* **release:** remove release-as pin so versions advance past 1.0.0 ([f769cf1](https://github.com/matt-riley/skill-evaluator/commit/f769cf15e76ab382e7bbb9e8a9a0933758016f99))
* restore cwd in tests with t.Chdir to stop getwd failures ([0a17eec](https://github.com/matt-riley/skill-evaluator/commit/0a17eec5f4463393b414d112c5cecafb447eddd1))
* use config/evals topology for benchmark aggregation ([e93b122](https://github.com/matt-riley/skill-evaluator/commit/e93b122f73b03d817e8a0f3432e62230ae330fdd))

## [1.0.0](https://github.com/matt-riley/skill-evaluator/compare/v0.6.0...v1.0.0) (2026-06-25)


### Features

* add --verbose flag and structured logging ([e6e41b9](https://github.com/matt-riley/skill-evaluator/commit/e6e41b9bf078e189420103821d701e07c1a3aa83))
* add cross-iteration delta to benchmark.json ([e85d0d7](https://github.com/matt-riley/skill-evaluator/commit/e85d0d7bb60471fa4925ee3516c81f0bf9e2262d))
* add deterministic file/text assertion matchers ([5011275](https://github.com/matt-riley/skill-evaluator/commit/5011275a3170355e0b6eede0a07302deeb1b31c3))
* add iteration lockfile and --resume ([b39f004](https://github.com/matt-riley/skill-evaluator/commit/b39f0046c257750fd01e7f982ec96a254290c626))
* validate config against JSON schema ([cf9b3f4](https://github.com/matt-riley/skill-evaluator/commit/cf9b3f4a5597908e6da4ff7cc27281629769d341))

## [0.6.0](https://github.com/matt-riley/skill-evaluator/compare/v0.5.0...v0.6.0) (2026-06-25)


### Features

* --models flag for cross-model benchmarking ([9fd5b45](https://github.com/matt-riley/skill-evaluator/commit/9fd5b4562e258dc70e72b58142eaf04d7ceff476))
* pre-push-guard pi extension ([500b005](https://github.com/matt-riley/skill-evaluator/commit/500b0052161deeea2e140c9f59e948b8859566ee))
* pre-push-guard shows live status widget while checks run ([ff5e658](https://github.com/matt-riley/skill-evaluator/commit/ff5e658fd4d0e7fd26e0827aa69c58abe5ff0981))


### Bug Fixes

* use &lt;video&gt; not ![]() for auto-fixing demo mp4 ([6d9c5e3](https://github.com/matt-riley/skill-evaluator/commit/6d9c5e3fc3f96c0e25d435fb793f090044a7e3bb))
* use &lt;video&gt; not ![]() for cross-model demo mp4 ([8cdceb8](https://github.com/matt-riley/skill-evaluator/commit/8cdceb8263e296441ad8b65112988d504647cb49))
* use agent:model pairs in all cross-model examples, regenerate VHS ([fd1a283](https://github.com/matt-riley/skill-evaluator/commit/fd1a2833b69f94223eaabfed0364f60354e73530))
* use proper &lt;video&gt;&lt;/video&gt; closing tags, agent:model pairs throughout cross-model guide ([8d3cbf5](https://github.com/matt-riley/skill-evaluator/commit/8d3cbf50660338c7ac77c6d9dae64e0ea5d39009))

## [0.5.0](https://github.com/matt-riley/skill-evaluator/compare/v0.4.0...v0.5.0) (2026-06-25)


### Features

* --fix flag for auto-refinement loop (evaluator-optimizer pattern) ([fa85108](https://github.com/matt-riley/skill-evaluator/commit/fa85108dd838c67de579526e30fb0e57d4cd73d8))


### Bug Fixes

* **a11y:** modern-web-guidance improvements ([3111adf](https://github.com/matt-riley/skill-evaluator/commit/3111adf52ef1c565c0b358847edc30a1f515dd6f))
* ignore err from CombinedOutput in fixEval (grading handles failures) ([e4fff21](https://github.com/matt-riley/skill-evaluator/commit/e4fff21e903e93ea5f1e8181ed6500e01a79a2ee))

## [0.4.0](https://github.com/matt-riley/skill-evaluator/compare/v0.3.0...v0.4.0) (2026-06-25)


### Features

* add view transitions for smooth page navigation ([bcfc382](https://github.com/matt-riley/skill-evaluator/commit/bcfc382f3daeb4f0322a58c4f19404445b9171ef))
* **docs:** reorder docs menu, rename README to Home, add GitHub link ([7d92c20](https://github.com/matt-riley/skill-evaluator/commit/7d92c20fd8453ceaa98082f3faff695beb4bb8e6))
* **docs:** wire full SEO stack ([7f82d89](https://github.com/matt-riley/skill-evaluator/commit/7f82d895424ec679d968e853324713bfceee80cd))


### Bug Fixes

* **docs:** compile images at build time to avoid 404 _image endpoint ([b8da6b2](https://github.com/matt-riley/skill-evaluator/commit/b8da6b2df63c541f9a7ca553de1acfc77e8cf0db))
* **docs:** put Home at the top of the docs menu ([8d42852](https://github.com/matt-riley/skill-evaluator/commit/8d42852ee6cfe04f4260a81ed3eecfb05ffb6504))
* **docs:** remove stale wrangler deploy config and add pages deploy script ([ea75a79](https://github.com/matt-riley/skill-evaluator/commit/ea75a79d7b7bc75d16268970415b5e81b9a328d7))
* menu layout, guide ordering, and larger GIFs ([c11bb0e](https://github.com/matt-riley/skill-evaluator/commit/c11bb0e9c824d298f8931cb869e6e7b684e531b7))
* VHS tapes use harness scripts, type real commands not comments ([7690633](https://github.com/matt-riley/skill-evaluator/commit/769063385927a9dfb1f901b25e28ce55f18a77e7))

## [0.3.0](https://github.com/matt-riley/skill-evaluator/compare/v0.2.0...v0.3.0) (2026-06-25)


### Features

* **docs:** add CSP header and optimize font loading ([b02a168](https://github.com/matt-riley/skill-evaluator/commit/b02a1689fed711496584bf5c19e1b747783f780c))
* **docs:** add mobile hamburger navigation ([4305673](https://github.com/matt-riley/skill-evaluator/commit/4305673c21e1b448dd98f09eef889b1d56160be4))
* migrate documentation to Astro site and modular markdown ([614e051](https://github.com/matt-riley/skill-evaluator/commit/614e0514100b612633126c414dd1b0592b7a8d4a))


### Bug Fixes

* **docs:** add custom 404 error page ([a6f2118](https://github.com/matt-riley/skill-evaluator/commit/a6f2118cf9ae551aa18d6c888da72ca1da4eb5e4))

## [0.2.0](https://github.com/matt-riley/skill-evaluator/compare/v0.1.0...v0.2.0) (2026-06-24)


### Features

* initial commit ([334c6ba](https://github.com/matt-riley/skill-evaluator/commit/334c6ba57497c6b470d28cea552cec0eb7f6a95a))
* initial implementation ([d13aa09](https://github.com/matt-riley/skill-evaluator/commit/d13aa09715c910f2c73b2e13584b7231dcb809d0))


### Bug Fixes

* resolve golangci-lint errors ([ac9abc3](https://github.com/matt-riley/skill-evaluator/commit/ac9abc32751cd65fe85897e61e94d12d54b6dc08))
