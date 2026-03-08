# Changelog

## [0.11.0](https://github.com/SentioLabs/arc/compare/v0.10.0...v0.11.0) (2026-03-08)


### Features

* agentic teams — role-aware context, team CLI, API endpoint, and web dashboard ([d648d6f](https://github.com/SentioLabs/arc/commit/d648d6fbec4e6dcc7243cc3e48b3ddda8aa3b0f5))
* **api:** add parent_id query parameter to ListIssues endpoint ([2a025d2](https://github.com/SentioLabs/arc/commit/2a025d299ff9a9f4ff0eca14456d37a01436741e))
* **api:** add review handler with in-memory session management ([5f9e950](https://github.com/SentioLabs/arc/commit/5f9e9502671fb04407ef17e91697f7b496402707))
* **api:** add review schemas and endpoints to OpenAPI spec ([c169ae9](https://github.com/SentioLabs/arc/commit/c169ae9ad85860f48b2be7defae8aa338c08eb42))
* **api:** add team-context endpoint ([5bf89d3](https://github.com/SentioLabs/arc/commit/5bf89d3041e95357bffc56dacd892b8a94fb8203))
* **api:** pass cascade to CloseIssue and return 409 on OpenChildrenError ([b9c43c3](https://github.com/SentioLabs/arc/commit/b9c43c3f56ba00972114254a74485268659d9622))
* **ci:** add nightly build workflow with 7-day cleanup ([1e09007](https://github.com/SentioLabs/arc/commit/1e0900721802217f33ad1d0c42b50d9538225a7c))
* **cli:** add --cascade flag to close command with formatted error output ([40b7002](https://github.com/SentioLabs/arc/commit/40b70026e5ce176bbcb561bd9843fec67afeb4d5))
* **cli:** add --parent flag to list command and Parent field to client ([b3379c5](https://github.com/SentioLabs/arc/commit/b3379c5d105875bc4cefb7747070d58eee9f91ed))
* **cli:** add --stdin flag to arc plan set ([134033a](https://github.com/SentioLabs/arc/commit/134033a64dcef115f2b06ffa04184e0b7aaefa9b))
* **cli:** add --title flag to arc create command ([2426ca7](https://github.com/SentioLabs/arc/commit/2426ca71f1f5c8f56aad4c997f6365d5cae2e7cc))
* **cli:** add arc self channel subcommand ([4e2045b](https://github.com/SentioLabs/arc/commit/4e2045b50fd94d5766cbf999940b682c7d47090c))
* **cli:** add arc team context command ([5c23176](https://github.com/SentioLabs/arc/commit/5c23176c221ab66f6241b3bb2a656e94b54b9b86))
* **cli:** add ARC_SERVER env var support for server URL resolution ([3439756](https://github.com/SentioLabs/arc/commit/3439756b1d085e2f1a418eead9649d4cea96aec2))
* **cli:** add channel field to config for update channels ([7319f56](https://github.com/SentioLabs/arc/commit/7319f5608f1a86793b4296d38215e876067095df))
* **cli:** add role-aware output to arc prime ([569a43e](https://github.com/SentioLabs/arc/commit/569a43e14ad393ab94249ca22d6a58b9e7e4ad57))
* **cli:** auto-detect stdin for descriptions in create and update ([fec2850](https://github.com/SentioLabs/arc/commit/fec28504598e67481f324100c25d1d5f59ae198c))
* **cli:** channel-aware version resolution with semver ([5d6cfcb](https://github.com/SentioLabs/arc/commit/5d6cfcb702a12acd11b8abe4b7fef05ba085bcfe))
* **client:** add cascade parameter to CloseIssue and parse 409 OpenChildrenError ([6ec4ed5](https://github.com/SentioLabs/arc/commit/6ec4ed5d08a89592fee29c3d15f456f5d897e77e))
* FTS5 full-text search, pre-migration backup, lint fixes, and remote access fix ([fe103f1](https://github.com/SentioLabs/arc/commit/fe103f172a5d232cc40806ca0191b96b8d74ba0f))
* **install:** add --tag parameter for specific version installs ([c8425c3](https://github.com/SentioLabs/arc/commit/c8425c3ddd39f0350f1e3da5d7003b932a6c2211))
* **plugin:** add arc team-deploy orchestration skill ([0018238](https://github.com/SentioLabs/arc/commit/00182383cba921b06d973844c6918400383b3872))
* **plugin:** add arc-implementer agent for TDD task execution ([5685e6a](https://github.com/SentioLabs/arc/commit/5685e6a9701ef184eaaa2396ff2c4ec838e3dcec))
* **plugin:** add arc-reviewer agent for code review dispatch ([414e0c8](https://github.com/SentioLabs/arc/commit/414e0c81cfde9ac81674278428cfdb72ab549461))
* **plugin:** add brainstorm skill for design discovery ([dd30400](https://github.com/SentioLabs/arc/commit/dd30400489c5945457b4ad00fe7c66dfff5f1fbd))
* **plugin:** add debug skill for systematic root cause investigation ([b6bf9b4](https://github.com/SentioLabs/arc/commit/b6bf9b4ecc5711f1bdd0abf7ce121ab2075fa4f9))
* **plugin:** add finish skill for unified session completion ([8ac9b9c](https://github.com/SentioLabs/arc/commit/8ac9b9c6862cc2eb833d9005b7ba25865f7078e4))
* **plugin:** add implement skill for subagent-driven TDD execution ([e93573b](https://github.com/SentioLabs/arc/commit/e93573bbdd2b4869af85c54045a0ace0f74774ae))
* **plugin:** add plan skill for implementation task breakdown ([f48234d](https://github.com/SentioLabs/arc/commit/f48234db089ff866e471e77c20f36fdcd0c8336e))
* **plugin:** add review skill for code review dispatch ([a483360](https://github.com/SentioLabs/arc/commit/a4833601e80d43706d10adebb4b265d6e7843218))
* **plugin:** add verify skill for evidence-based completion gates ([d9c68db](https://github.com/SentioLabs/arc/commit/d9c68db91d65081a2e5c4e1d78239931db60375b))
* **plugin:** route doc-only tasks to skip TDD ([826861a](https://github.com/SentioLabs/arc/commit/826861aeccc483b05a627fe9156066ce50414a51))
* remove web-based diff review feature ([ab9a500](https://github.com/SentioLabs/arc/commit/ab9a50078cfda2720ce3ffce514853e349b0baf3))
* **review:** add diff parsing utility with diff2html ([b9d7e52](https://github.com/SentioLabs/arc/commit/b9d7e523bf6dbefae5c67ef7ca24b347aa95d725))
* **review:** add line comment gutter button and inline comment form ([4869dc0](https://github.com/SentioLabs/arc/commit/4869dc087dd126612384e96f32a0e6d275a9b75d))
* **review:** add line_comments to review API schema and handler ([f9c3837](https://github.com/SentioLabs/arc/commit/f9c38378a459f6d9ec475a17749a71f511388798))
* **review:** add resizable sidebar with drag handle and localStorage persistence ([ad80ddc](https://github.com/SentioLabs/arc/commit/ad80ddc49124e876014b887d741ca11630070802))
* **review:** add Shiki-based syntax highlighting utility for diff lines ([6cb18a8](https://github.com/SentioLabs/arc/commit/6cb18a8827cb17f9982749d673a082c3e12e39a2))
* **review:** add tooltip, horizontal scroll, and remove truncation in FileTree ([e80f1cc](https://github.com/SentioLabs/arc/commit/e80f1cc6043967d884bd2ecb8d9341bff2ee70c8))
* **review:** create DiffLine and FileSection components ([ede8e2a](https://github.com/SentioLabs/arc/commit/ede8e2a7a2191b2f154800d4d73abda4e98728c1))
* **search:** add FTS5 full-text search, pre-migration backup, and lint fixes ([d75d109](https://github.com/SentioLabs/arc/commit/d75d109361f3f3f34e787ff9d9c50ece5401e79b))
* **storage:** add cascade parameter to CloseIssue with guard and recursive close ([1a4f9b4](https://github.com/SentioLabs/arc/commit/1a4f9b4dcfefe7613cc562d52fc24f5858bb5ebc))
* **storage:** add OpenChildrenError type and GetOpenChildIssues query ([773e104](https://github.com/SentioLabs/arc/commit/773e10474625452c06df94e2360e99f2d246ca96))
* **storage:** add ParentID filter to IssueFilter and ListIssuesByParent query ([07b5591](https://github.com/SentioLabs/arc/commit/07b5591ffd19215a37f31a9e430aaede7f1089ec))
* **web:** add diff2html dependency and review API client functions ([a033fd5](https://github.com/SentioLabs/arc/commit/a033fd5cc232c3f35da34d62697e4f1c571ffd17))
* **web:** add inline editing to issue detail page ([e07ecd6](https://github.com/SentioLabs/arc/commit/e07ecd61b2c7620a539bc01ba6daec80626a5961))
* **web:** add markdown rendering with syntax highlighting ([2fbc832](https://github.com/SentioLabs/arc/commit/2fbc832623eb6e27aebc7d08a9a6d4251372c47c))
* **web:** add quick status toggle to issue list page ([228c13f](https://github.com/SentioLabs/arc/commit/228c13f5ae24246f02b6482a1a64b9ec8fe6f670))
* **web:** add ReviewPage route and wire review nav link ([eea2c0d](https://github.com/SentioLabs/arc/commit/eea2c0d46c0f12cad3024e376af4aacd7b31ea7b))
* **web:** add team context API client function ([5dbf2fe](https://github.com/SentioLabs/arc/commit/5dbf2fed425317b465c3e5b00b9838fda2893725))
* **web:** add Team View dashboard page ([0c3b929](https://github.com/SentioLabs/arc/commit/0c3b929d274fc73c3d0528e0c632bd9d1f4ae3bb))


### Bug Fixes

* **cli:** match rc tags without dot separator and silence usage on errors ([7fdcaf3](https://github.com/SentioLabs/arc/commit/7fdcaf365d6b9bff8347066f4727021b9d66852a))
* **cli:** resolve non-stable channels against stable releases too ([d42f03e](https://github.com/SentioLabs/arc/commit/d42f03ebb3be4ce9e38ac576ad401a1fcc57a262))
* **cli:** use ListIssues with parent filter in team context for label support ([9a873f1](https://github.com/SentioLabs/arc/commit/9a873f1df86c339001757d21005c0795f39ab4ae))
* **cli:** validate channel name before prompting for confirmation ([b673f29](https://github.com/SentioLabs/arc/commit/b673f2975274d5e5c2fa1efbd6003c57e0c51362))
* **plugin:** add AskUserQuestion guidance, remove -w flag, clean up stale review phase ([5f34e19](https://github.com/SentioLabs/arc/commit/5f34e196a70fb0c72092d5bb8f9b438856852138))
* **plugin:** add missing frontmatter and fix arc close flag in skills ([8f24c5c](https://github.com/SentioLabs/arc/commit/8f24c5c674bd4284684b19f95cf3b1a78fe846b4))
* **plugin:** audit and harden skills and agents for reliability ([0fd99cd](https://github.com/SentioLabs/arc/commit/0fd99cdbcdb05586822c8b8317a98d7a9fd564e0))
* **review:** add aria-label to gutter button ([854f1a3](https://github.com/SentioLabs/arc/commit/854f1a324ed6c9e96611fa96894cc7ab74304c54))
* **web:** address code review findings for inline editing components ([2371799](https://github.com/SentioLabs/arc/commit/2371799b1cf8753724bc812601b5e302f5543dfc))
* **web:** enable horizontal scrolling for long diff lines ([027de68](https://github.com/SentioLabs/arc/commit/027de6871ff258e2f6a64f0c6b8389c249c3b4e5))
* **web:** resolve all svelte-check errors ([f90beaa](https://github.com/SentioLabs/arc/commit/f90beaabcb93d4a88398088bd86017e1609a0eba))
* **web:** resolve svelte-check warnings in inline edit components ([b86c81c](https://github.com/SentioLabs/arc/commit/b86c81c3090bcd37b294a525fc4ee8f8a83cd24b))


### Performance

* **plugin:** parallelize bulk issue creation in arc-issue-tracker ([e9013de](https://github.com/SentioLabs/arc/commit/e9013de0fada6d6630a3deb20cc425dc5bd29cd2))


### Refactoring

* **cli:** use text/template for prime output ([249bc92](https://github.com/SentioLabs/arc/commit/249bc92eab8b5401f96e3e53d0fedec7106bb040))
* remove all .arc.json support in favor of ~/.arc/projects/ ([3071584](https://github.com/SentioLabs/arc/commit/3071584d95f5af2a6f8c66ba30067eecdcd6a83f))
* **review:** use action for autofocus, remove unused eslint directive ([713ecac](https://github.com/SentioLabs/arc/commit/713ecac6c46fda3bd1cb0fe77521f2e2c08354c7))

## [0.10.0](https://github.com/SentioLabs/arc/compare/v0.9.0...v0.10.0) (2026-02-28)


### Features

* add --prefix flag to arc init for custom issue prefixes ([acaab9b](https://github.com/SentioLabs/arc/commit/acaab9b0e77e98d9126acf6cf393e8c5c3011709))
* add --prefix flag to arc init for custom issue prefixes ([de23623](https://github.com/SentioLabs/arc/commit/de23623ba3a8ef3eddcbb915dbdfbfd1e769e809))
* add ColorPicker component with preset palette ([86fe274](https://github.com/SentioLabs/arc/commit/86fe274a36bc1a6b5b59b0f0439b896c13f151bf))
* add GeneratePrefixWithCustomName for custom prefix support ([60060ab](https://github.com/SentioLabs/arc/commit/60060abe88b4d5adb6e20e63a597e3eeb9d1a1e7))
* add global label CRUD functions to frontend API client ([0c5d5f9](https://github.com/SentioLabs/arc/commit/0c5d5f98b01ed3eb67c6c4b0774f5cf1c0c86674))
* add global labels management page with color picker ([4789f11](https://github.com/SentioLabs/arc/commit/4789f11ebb6c00db29ac210cff9c54393ae5c492))
* global labels with color picker ([0e01743](https://github.com/SentioLabs/arc/commit/0e01743ed7e8cdc0052cd8235d99d301a47d038b))
* increase prefix basename length from 5 to 10 characters ([ac84e4a](https://github.com/SentioLabs/arc/commit/ac84e4a170bebd4edb65bd740d1fa898fb5fb3ad))
* increase workspace prefix max length from 10 to 15 ([7d813f5](https://github.com/SentioLabs/arc/commit/7d813f58bc06e328f89892d7fc12331779dd9985))
* migration to make labels global (drop workspace_id) ([b554e9d](https://github.com/SentioLabs/arc/commit/b554e9de7131fb43565e5aa654ab85303b2a7bcb))
* move label CRUD endpoints to global scope ([7e6d373](https://github.com/SentioLabs/arc/commit/7e6d3731e9d9d3d6e191553b129f9e4610e1378e))
* render colored label badges on issue cards ([9397950](https://github.com/SentioLabs/arc/commit/93979501dedc6d9669e9bc688fb25aa262a915cd))
* update Label type and storage interface for global labels ([4ab6e58](https://github.com/SentioLabs/arc/commit/4ab6e586e1025634fd662b9e4265fb93d8beb4d4))
* update OpenAPI spec for global labels ([7440c70](https://github.com/SentioLabs/arc/commit/7440c70762402e81650bb777dbd417d0e462558f))
* update sqlc queries for global labels ([1585a85](https://github.com/SentioLabs/arc/commit/1585a85ea79fc4570651c9b94191e08a25863a45))
* update SQLite label storage for global labels ([848e920](https://github.com/SentioLabs/arc/commit/848e92084827784945d7f3785533db71882780d5))
* **web:** add hybrid Biome + ESLint/Prettier linting setup ([3ea716e](https://github.com/SentioLabs/arc/commit/3ea716e0a0d14779f42a11589f1ef3c1ff3d9240))
* **web:** redesign ColorPicker with curated presets and native color wheel ([ea6cf40](https://github.com/SentioLabs/arc/commit/ea6cf40305bdf9763ecd3312184a8833127c601e))


### Bug Fixes

* fix escaped dollar sign in Sidebar.svelte ([ecad7c5](https://github.com/SentioLabs/arc/commit/ecad7c54fd94e1275f7eea07f60d08a162da86d4))
* resolve all golangci-lint errors across codebase ([9628ee9](https://github.com/SentioLabs/arc/commit/9628ee99efa380e4c24cc7013949a8783f8447f7))
* resolve all golangci-lint errors and upgrade to Go 1.26 ([c0e9fa4](https://github.com/SentioLabs/arc/commit/c0e9fa4b740dcd87cb767f447fa6f30bfd47098c))
* resolve Svelte 5 autofixer issues in label components ([fe10ceb](https://github.com/SentioLabs/arc/commit/fe10cebdc876ff40990e4860f0dc445a7f594df0))
* **web:** resolve Biome lint warnings in API client and filter store ([1a1db3d](https://github.com/SentioLabs/arc/commit/1a1db3d4859648ec7b1b0d987b00670fbb805377))

## [0.9.0](https://github.com/SentioLabs/arc/compare/v0.8.1...v0.9.0) (2026-02-27)


### Features

* arc init writes to ~/.arc/projects/ instead of .arc.json ([71550fd](https://github.com/SentioLabs/arc/commit/71550fddd5e136ce2d87df793477ce1dc23a22cc))
* arc which shows project config path ([4a31886](https://github.com/SentioLabs/arc/commit/4a3188602b26359e75c876ce09c5d137890e1763))
* clean up project configs on workspace delete ([01bb0eb](https://github.com/SentioLabs/arc/commit/01bb0eb2a7afd8be1e7dc74e5faab3c0d62faebe))
* project-local workspace resolution via ~/.arc/projects/ ([acee3c5](https://github.com/SentioLabs/arc/commit/acee3c5c857afd1ab31fc44b5b2c031a6b19b2c6))
* **project:** add config read/write for ~/.arc/projects/ ([6c092fb](https://github.com/SentioLabs/arc/commit/6c092fba5bffb3d0630ceb710a87caa172e248f9))
* **project:** add legacy .arc.json migration ([82e38cd](https://github.com/SentioLabs/arc/commit/82e38cdbf788146464aec8082812966bc1a26d44))
* **project:** add path-to-project-dir conversion ([ff7eaeb](https://github.com/SentioLabs/arc/commit/ff7eaebe75156558fe53474380689e192ac90ece))
* **project:** add project root resolution (git walk + prefix walk) ([ea8d5af](https://github.com/SentioLabs/arc/commit/ea8d5af8e1bc5755c519dd5fea9807f93f60dd96))
* update workspace resolution to use ~/.arc/projects/ ([fb8e333](https://github.com/SentioLabs/arc/commit/fb8e3331235d0f9d0585b0d7c9db728c8f780077))

## [0.8.1](https://github.com/SentioLabs/arc/compare/v0.8.0...v0.8.1) (2026-02-27)


### Bug Fixes

* guard WebUI URL for ephemeral ports and CLI-only builds ([e1edac3](https://github.com/SentioLabs/arc/commit/e1edac3672b1af10abb710257188315a5adbd3c3))

## [0.8.0](https://github.com/SentioLabs/arc/compare/v0.7.1...v0.8.0) (2026-02-27)


### Features

* add port to server status output ([6fb18ff](https://github.com/SentioLabs/arc/commit/6fb18ffd4f63875682599adcd4b8a940c883990d))

## [0.7.1](https://github.com/SentioLabs/arc/compare/v0.7.0...v0.7.1) (2026-01-24)


### Bug Fixes

* bump release ([e6d4a4b](https://github.com/SentioLabs/arc/commit/e6d4a4b30e4e60f7333118b3f14b005ea4e1d765))

## [0.7.0](https://github.com/SentioLabs/arc/compare/v0.6.1...v0.7.0) (2026-01-24)


### Features

* **plans:** add hybrid plan feature with comprehensive test coverage ([a923410](https://github.com/SentioLabs/arc/commit/a9234105456c4d3e6ff27e3372e0ebc72e0c95ba))

## [0.6.1](https://github.com/SentioLabs/arc/compare/v0.6.0...v0.6.1) (2026-01-23)


### Bug Fixes

* removing files ([5816f49](https://github.com/SentioLabs/arc/commit/5816f49b6c6b39a7dc88b9ec8dcdb31289ec315d))

## [0.6.0](https://github.com/SentioLabs/arc/compare/v0.5.0...v0.6.0) (2026-01-23)


### Features

* **cli:** add docs search with BM25 ranking and fuzzy matching ([2c0d87b](https://github.com/SentioLabs/arc/commit/2c0d87b9bacbe6d925ba6c71d18a416f26dcc8fb))

## [0.5.0](https://github.com/SentioLabs/arc/compare/v0.4.1...v0.5.0) (2026-01-23)


### Features

* **cli:** add config command and auto-create cli-config.json ([288c5af](https://github.com/SentioLabs/arc/commit/288c5afb82de41674fcbe94a21e9db56dcc525cf))

## [0.4.1](https://github.com/SentioLabs/arc/compare/v0.4.0...v0.4.1) (2026-01-23)


### Refactoring

* consolidate config to ~/.arc/ and improve file naming ([0a2d24f](https://github.com/SentioLabs/arc/commit/0a2d24f15b9c2149244155579a47ba86757b2e26))

## [0.4.0](https://github.com/SentioLabs/arc/compare/v0.3.2...v0.4.0) (2026-01-23)


### Features

* **web:** add reactive search and custom Select component ([dd5e873](https://github.com/SentioLabs/arc/commit/dd5e873b6933764ce28463be30b95b742d71c79f))
* **web:** add workspace deletion with batch support and Playwright tests ([18d7ec5](https://github.com/SentioLabs/arc/commit/18d7ec50d118b0e9958043aaa4da7b1563d07bd8))


### Bug Fixes

* **web:** add ESLint + Prettier linting setup ([cf310cb](https://github.com/SentioLabs/arc/commit/cf310cb82e633f5fb147d2577c293875a85575a2))

## [0.3.2](https://github.com/SentioLabs/arc/compare/v0.3.1...v0.3.2) (2026-01-22)


### Bug Fixes

* find .arc.json in parent directories for subdirectory support ([d405ae7](https://github.com/SentioLabs/arc/commit/d405ae729cb281a91dd4599502b3ce72fcda8246))

## [0.3.1](https://github.com/SentioLabs/arc/compare/v0.3.0...v0.3.1) (2026-01-22)


### Bug Fixes

* append session completion reference to existing CLAUDE.md without one ([6c05fd0](https://github.com/SentioLabs/arc/commit/6c05fd07d3b4a41cb2f3466045951bb2d987c5fb))

## [0.3.0](https://github.com/SentioLabs/arc/compare/v0.2.1...v0.3.0) (2026-01-21)


### Features

* add hash-based workspace prefixes for guaranteed uniqueness ([5b85364](https://github.com/SentioLabs/arc/commit/5b8536480bc27a81102340374868536804b82748))
* change issue ID separator from hyphen to period ([74be6b8](https://github.com/SentioLabs/arc/commit/74be6b830768038d47aac73d725a284806b888e0))


### Bug Fixes

* **ci:** add write permissions for plugin version workflow ([2314cb1](https://github.com/SentioLabs/arc/commit/2314cb1b3e01670b704b8faf2f523d9c943414bf))
* restart server after update if it was running ([856c51a](https://github.com/SentioLabs/arc/commit/856c51a9f6d6862854edeabef55b7ccf4816a7b1))

## [0.2.1](https://github.com/SentioLabs/arc/compare/v0.2.0...v0.2.1) (2026-01-21)


### Bug Fixes

* include self-update simplification in release ([9ef9d10](https://github.com/SentioLabs/arc/commit/9ef9d1041303afe488529520bc14da6f37ecf1eb))

## [0.2.0](https://github.com/SentioLabs/arc/compare/v0.1.1...v0.2.0) (2026-01-21)


### Features

* add self-update command and improve installer ([4c4a1d3](https://github.com/SentioLabs/arc/commit/4c4a1d361da11b8bd292034cca2cd9f73aff43a0))
* improve arc workspace list with table formatting ([96718ca](https://github.com/SentioLabs/arc/commit/96718cadc8ac69258d21dae50826a9f9c0942e07))


### Bug Fixes

* parent-child deps should not block issues from ready list ([2ca6067](https://github.com/SentioLabs/arc/commit/2ca6067b591666d8e7493e11dbd3417aec45503e))
* use version package in server health endpoint ([45c6bd5](https://github.com/SentioLabs/arc/commit/45c6bd5ffc370d8630da471041bde98657ae88ed))

## [0.1.1](https://github.com/SentioLabs/arc/compare/v0.1.0...v0.1.1) (2026-01-20)


### Bug Fixes

* combine release-please and goreleaser into single workflow ([54d302f](https://github.com/SentioLabs/arc/commit/54d302f9fac1b2bbb857d5e104239fbe995807a0))

## 0.1.0 (2026-01-20)


### Features

* add agent integration and Claude Code support ([3fcac6e](https://github.com/SentioLabs/arc/commit/3fcac6e9ad0d84593588ed17cd750d52658a8a18))
* add arc-issue-tracker agent for bulk operations ([71f5d02](https://github.com/SentioLabs/arc/commit/71f5d02940f3f388b5afda2c187e756d949e9d54))
* add description, notes, and acceptance-criteria flags to arc update ([fc98834](https://github.com/SentioLabs/arc/commit/fc988347d46a838c35898df6a47698b63571c49f))
* add goreleaser CI/CD and remove notes/acceptance_criteria fields ([5d1317e](https://github.com/SentioLabs/arc/commit/5d1317efc2cb4784ce5056af90b0c086dbc37030))
* add ranked ordering for arc ready command ([870d2f3](https://github.com/SentioLabs/arc/commit/870d2f3ef630b76c80d48b587b6bb2b7da2e21c6))
* add Release Please for automated versioning ([f5d30bb](https://github.com/SentioLabs/arc/commit/f5d30bb193ebd2cf2c923ecdb1c4610a48e713ff))
* add Svelte web UI with OpenAPI-first backend refactoring ([b2698ed](https://github.com/SentioLabs/arc/commit/b2698edd64247de6059eebd0b23423c7a3896f0c))
* rename project from beads-central to arc ([8704d4f](https://github.com/SentioLabs/arc/commit/8704d4f1f3886bc5fe0f97893838ce282b31b23e))
* unify arc CLI and server into single binary with daemon management ([81660bd](https://github.com/SentioLabs/arc/commit/81660bd6e14aee49e768a7be6203b6eca8843a0e))


### Bug Fixes

* add workspace validation to prevent cross-workspace IDOR ([7a1c2fa](https://github.com/SentioLabs/arc/commit/7a1c2fa057ecdb9b7bf6d86a048a4b19cd362895))
* defer rank column/index creation to migrations ([a9ab0c3](https://github.com/SentioLabs/arc/commit/a9ab0c34e8ee1cb70f60e001496bf52cb8e50d29))
* fail fast on missing workspace config instead of silent fallback ([24d1abf](https://github.com/SentioLabs/arc/commit/24d1abf266fb530950c0b725c2be23bb35161d45))
* handle empty workspace stats gracefully ([5899917](https://github.com/SentioLabs/arc/commit/5899917b1e368df3eb866e420cb32c29b0c4fc6c))
* replace bd references with arc in prime command output ([075f2b5](https://github.com/SentioLabs/arc/commit/075f2b5735016f11530ee5ff71c54e94f1197a4e))
