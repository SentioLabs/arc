# Changelog

## [0.13.0](https://github.com/SentioLabs/arc/compare/v0.12.0...v0.13.0) (2026-03-26)


### Features

* add migration 014 to simplify plans schema ([e6057ff](https://github.com/SentioLabs/arc/commit/e6057ff8986d1f29e20579826f094e4d9935e25c))
* add plan API client methods for ephemeral model ([150edb8](https://github.com/SentioLabs/arc/commit/150edb8cf194ac6712f50bf14cebb223a6ae469d))
* add plan comments command, remove unused editInEditor helper ([1db6425](https://github.com/SentioLabs/arc/commit/1db6425c13ffd6ed62657169b3ef543831cb3899))
* add planner web UI for ephemeral plan review ([3727782](https://github.com/SentioLabs/arc/commit/3727782f9730d60beba0924911d46691a2a1ea8b))
* add TaskCreate progress tracking to implement skill ([ef1afd0](https://github.com/SentioLabs/arc/commit/ef1afd04408310470b574ab272f1d0636055edee))
* AI agent observability — session and subagent tracking with transcript viewer ([2719b78](https://github.com/SentioLabs/arc/commit/2719b7844b77bc29290d98f8d418601b0e0a332a))
* **api:** include agent_summary in session list response ([bb09edf](https://github.com/SentioLabs/arc/commit/bb09edf3f5a519153e93fbd2ef3c7c41d3cab498))
* **api:** move AI session endpoints under project scope ([3529bbc](https://github.com/SentioLabs/arc/commit/3529bbcb25ee2f8344f9cc8c1ce73526d09f2afc))
* **api:** parse repeated query params for multi-value issue filters ([9d14525](https://github.com/SentioLabs/arc/commit/9d145255ce4e42f27fb673d4b843eaa9b4a45f0e))
* **api:** remove assignee from OpenAPI spec and handlers ([b6afddb](https://github.com/SentioLabs/arc/commit/b6afddbd8ce898a7cda775353091e0ff05a2a979))
* **api:** update OpenAPI spec for project-scoped AI sessions ([d5a4917](https://github.com/SentioLabs/arc/commit/d5a491703d9c38a54e488cbc4c34b6816b938b6d))
* **api:** update plan handlers for unified storage with status workflow ([985a071](https://github.com/SentioLabs/arc/commit/985a07127d85b98a066cffa741ce9716c502563a))
* **cli:** add --stdin flag to arc ai session start ([5258061](https://github.com/SentioLabs/arc/commit/5258061ba0fcd6620986ea0502f0ee4d82496f63))
* **cli:** add label management commands and integrate with create/update ([5e5871c](https://github.com/SentioLabs/arc/commit/5e5871c9d72c7f5bd1fa179c6ba2ad8935e162e5))
* **client:** update plan client methods for unified storage ([88c0900](https://github.com/SentioLabs/arc/commit/88c0900510f783da04a2b231fbc2250febc43a13))
* **cli:** remove assignee flags and unassigned from team context ([5689255](https://github.com/SentioLabs/arc/commit/56892558031f25188d91de8a7f5efeee35d01e7c))
* **cli:** rewrite plan commands for unified storage with approve/reject ([79281c7](https://github.com/SentioLabs/arc/commit/79281c742ebe980fbf85f194d61c8828b3a3ab1e))
* **cli:** show pending plan review count in arc show and arc ready ([1289f0f](https://github.com/SentioLabs/arc/commit/1289f0fa7f21c3f4b344e03abdf884815af4b9e0))
* **cli:** update client methods and session start to use project-scoped AI session paths ([94eaa3c](https://github.com/SentioLabs/arc/commit/94eaa3cae48b211f57d5c7621c8aab1fa3f68ded))
* **evals:** add assertions for contract tests and plan adherence ([0af284e](https://github.com/SentioLabs/arc/commit/0af284e3045a432c0d6c641e1c1093b9aa1951fb))
* **plans:** add global plans API, issue linking, and frontend client ([bb2ab7c](https://github.com/SentioLabs/arc/commit/bb2ab7c3750770b0c688de03e5cbd19759a92189))
* **plugin:** arc-reviewer checks plan adherence against design spec ([7a217c8](https://github.com/SentioLabs/arc/commit/7a217c8a7b3ab08fd66ab433df11760d9cc21b4a))
* **plugin:** brainstorm skill produces exact code contracts ([4eda0de](https://github.com/SentioLabs/arc/commit/4eda0dea5f43f5e95ff6750e4499009c4ac2d199))
* **plugin:** implement skill requires review with plan adherence after every task ([53256e2](https://github.com/SentioLabs/arc/commit/53256e24826cf1af36991b217daab9df31b94c7b))
* **plugin:** plan skill generates contract tests in T0 and design contracts in task descriptions ([cce4c1b](https://github.com/SentioLabs/arc/commit/cce4c1b828d80ff5d78ebb9b4f1b60622d7fc1c1))
* **plugin:** review skill passes design excerpt for plan adherence ([f308a18](https://github.com/SentioLabs/arc/commit/f308a1875d471bad459f8a8a3a74e43b967ed848))
* remove old plan web UI pages and sidebar references ([e7cd041](https://github.com/SentioLabs/arc/commit/e7cd041f5c330470d781781209fd95afddbfa680))
* replace Plan types with simplified ephemeral plan model ([1ddb7d4](https://github.com/SentioLabs/arc/commit/1ddb7d4a025e9d369866f4cab02278d58dab9f45))
* rewrite integration tests for ephemeral plan system ([1c1d947](https://github.com/SentioLabs/arc/commit/1c1d947fd5e68a1ad9fcb968c34fd943a10fe801))
* rewrite plan API handlers for ephemeral model ([15cf6cf](https://github.com/SentioLabs/arc/commit/15cf6cf5c77a5ca2d9fe9c5c658823f83d7bd16f))
* rewrite plan HTTP client for ephemeral model ([cae8de3](https://github.com/SentioLabs/arc/commit/cae8de37ea759ccc2d89a82536d2519f727db5f2))
* rewrite plan storage implementation for ephemeral model ([934c1b4](https://github.com/SentioLabs/arc/commit/934c1b4cc2a1eb48259e6ff7b58633446fb8d155))
* simplify plan storage interface and sqlc queries ([aa34c36](https://github.com/SentioLabs/arc/commit/aa34c36fc3ea27a722d9e3ef62804ee5d8986bb7))
* **storage:** add project_id to ai_sessions with migration ([8ff0122](https://github.com/SentioLabs/arc/commit/8ff0122afa9e0b9e84786672f831196f47cb88d4))
* **storage:** drop assignee column and remove from types/queries ([f42fa9a](https://github.com/SentioLabs/arc/commit/f42fa9a8cc3318e5335ae7e4c6a228d4db39da14))
* **storage:** implement GetAgentSummariesForSessions query ([65fed28](https://github.com/SentioLabs/arc/commit/65fed284329ff09aee662719605bb8c3d441198f))
* **storage:** update ListIssues for multi-value status/type/priority filters ([5bbde7f](https://github.com/SentioLabs/arc/commit/5bbde7f69d1e31ef3d0deccdbf08a87f14692887))
* **types:** add AgentSummary type and storage interface for session dashboard ([3380749](https://github.com/SentioLabs/arc/commit/33807495005fded0f8f98eab8d7ae6872a23e387))
* **types:** convert IssueFilter to multi-value slices for status, type, priority ([cf6825c](https://github.com/SentioLabs/arc/commit/cf6825c08d0a9a97554e6b84f6e694b6b32c3643))
* update OpenAPI spec for simplified plan endpoints ([4708c3e](https://github.com/SentioLabs/arc/commit/4708c3e9111eef8a442c77724b558e1f3db0b67d))
* use TaskCreate for visible progress tracking in brainstorm/plan skills ([aa47ad4](https://github.com/SentioLabs/arc/commit/aa47ad419882b69252270b8a7b07f1ba6d7fa3e6))
* **web:** add agent transcript viewer page ([aa8590c](https://github.com/SentioLabs/arc/commit/aa8590c69ea34f14d37acfb9be5d16d23d5e6ca3))
* **web:** add copy-to-clipboard button for issue IDs ([26dc5f5](https://github.com/SentioLabs/arc/commit/26dc5f59656ab1618c11ecedf48e4443e8cf423e))
* **web:** add delete and batch delete for AI sessions ([7c24458](https://github.com/SentioLabs/arc/commit/7c2445862b6bd6f0b1258ad5e7c0c2e2d2e1be01))
* **web:** add draft plan count badge to sidebar Plans nav item ([9dffbfc](https://github.com/SentioLabs/arc/commit/9dffbfc0dcfdefe7731285efc3258d432593958a))
* **web:** add global plans list page ([f06884d](https://github.com/SentioLabs/arc/commit/f06884d969cd0a88c02940495a5fdb9ba827dece))
* **web:** add IssuePicker modal component with live search ([8de99d6](https://github.com/SentioLabs/arc/commit/8de99d6ef12ee21e1bf460744b85f58ac4746e67))
* **web:** add MultiSelect component with chips and dropdown ([825929f](https://github.com/SentioLabs/arc/commit/825929fd5cfa2f8624e58cc5fd82e9c655f2499a))
* **web:** add pagination and page size controls to AI sessions ([a20f718](https://github.com/SentioLabs/arc/commit/a20f7186af64513d0be6e8040763512620e04037))
* **web:** add plan detail page with action menu ([c72d63c](https://github.com/SentioLabs/arc/commit/c72d63c15778891196231506b4bb80e90e53724d))
* **web:** add Plans navigation entries to sidebar ([eb38836](https://github.com/SentioLabs/arc/commit/eb38836188119d1f5b61d46507f6cc10ca1cb879))
* **web:** add project-scoped plans list page ([bf4ed95](https://github.com/SentioLabs/arc/commit/bf4ed952a715ec9ec3055a9742d1de38b84d8166))
* **web:** add Recent AI Sessions dashboard widget to project page ([a0370c8](https://github.com/SentioLabs/arc/commit/a0370c8dc4f71593348dae8f942284bcc5df9f11))
* **web:** add split-pane plan editor with approve/reject workflow ([8cb55b9](https://github.com/SentioLabs/arc/commit/8cb55b9eea483d7889f3d2c6e71de239c52862f7))
* **web:** integrate MultiSelect filters on issues page ([474eef7](https://github.com/SentioLabs/arc/commit/474eef75f2ac148d2b01ff208dc189b79db3d0c3))
* **web:** move AI sessions under project-scoped routes ([dece22b](https://github.com/SentioLabs/arc/commit/dece22b6825948c8bf570f216bd971204b4561cb))
* **web:** redesign planner with tabbed view modes ([ecbfca7](https://github.com/SentioLabs/arc/commit/ecbfca717f39d5f2fb3041237465ded9a69a0977))
* **web:** remove assignee from components, pages, and filters ([b3ffd0c](https://github.com/SentioLabs/arc/commit/b3ffd0c797ad51f5a59e7b12b08fc7433192518a))
* **web:** route draft plans to editor, others to read-only viewer ([5eaae55](https://github.com/SentioLabs/arc/commit/5eaae55c5e89bec18cbd8fc09a65644e148e843e))
* **web:** show total tokens in AI session detail header ([3f22f88](https://github.com/SentioLabs/arc/commit/3f22f881e4bb1a93c0d7af176052f70d8a8b3bba))


### Bug Fixes

* align nolint directives with CI golangci-lint v2.11.x ([efe71ba](https://github.com/SentioLabs/arc/commit/efe71bac2087b54eb5964d65f4b5e088bece6b5f))
* **api:** add project-agnostic issue lookup endpoint ([d84c8fa](https://github.com/SentioLabs/arc/commit/d84c8fabc0573f41216a1d90fc029dc75506c5c6))
* **api:** filter progress entries before message check in normalization ([21f2605](https://github.com/SentioLabs/arc/commit/21f2605d392477601e73a65ff34e642c62ea4c83))
* **api:** make all issue-scoped commands work without project context ([4c9a614](https://github.com/SentioLabs/arc/commit/4c9a61436adb0bf5d309afefa3535aef3541721f))
* **api:** normalize transcript entries for frontend consumption ([51dca05](https://github.com/SentioLabs/arc/commit/51dca057e1e24a1de428f258f5725132110b6b63))
* **api:** restore dedicated getIssueByID handler for naming consistency ([7dbfb23](https://github.com/SentioLabs/arc/commit/7dbfb23a7013d08da9a71d930977f0399b809091))
* **api:** trim project-agnostic routes to only those used by CLI ([bf0a4ed](https://github.com/SentioLabs/arc/commit/bf0a4ed128efc57a3c4121db0321c64e6990dc69))
* **api:** use paginated response for listAISessions endpoint ([660c892](https://github.com/SentioLabs/arc/commit/660c89288ab3215d29ade70d702e1a6fa5d52b86))
* **cli:** use flag-changed detection for label updates and fix JSON output ([d79cdde](https://github.com/SentioLabs/arc/commit/d79cddee23e63b830dc0847e0062df8acf9b5753))
* don't fail hook when AI session creation fails for unregistered projects ([5ff4a38](https://github.com/SentioLabs/arc/commit/5ff4a383e7214445ab3429b5a4cecae796e70692))
* ensure planner URL is always shown, clean up skill transitions ([aaea716](https://github.com/SentioLabs/arc/commit/aaea716069c2ae5241bd4d26d74cbea87c73a444))
* **integration:** create project before using project-scoped AI session endpoints ([13e3578](https://github.com/SentioLabs/arc/commit/13e3578d64e3036b300c1d802bba1bd5a2569995))
* **lint:** resolve goconst, perfsprint, revive lint issues ([99502e7](https://github.com/SentioLabs/arc/commit/99502e7f2c5650e67a1e4bbf6eb97d4015e4163e))
* plan skill must write full design into epic description ([4734f67](https://github.com/SentioLabs/arc/commit/4734f67a3fde394e61fc2352611988988ad9374d))
* planner URL in AskUserQuestion options, correct CLI syntax ([a79c0ab](https://github.com/SentioLabs/arc/commit/a79c0ab07145290e5fbf5b8187ddbd126959d81d))
* remove duplicate arcCmdWithStdin from plan_test.go ([b1e8353](https://github.com/SentioLabs/arc/commit/b1e835357e38f66ec9a79abb8a0db7e1215826d0))
* remove remaining old Plan type references from types_test.go ([94e4c6b](https://github.com/SentioLabs/arc/commit/94e4c6bafc15e1ccee8be9fbb7fcec5c661a6b0c))
* resolve all frontend lint errors and add bun lint to make lint ([29794e5](https://github.com/SentioLabs/arc/commit/29794e5ac2d66dfb2037c035eaf77c9bfa45a37a))
* resolve all golangci-lint errors ([66ddb1c](https://github.com/SentioLabs/arc/commit/66ddb1c5b3df3fd62acddd295ff036b1fc20242f))
* resolve all golangci-lint issues across codebase ([f0e2cb7](https://github.com/SentioLabs/arc/commit/f0e2cb7db8a47139d42746149bb907a56788fd44))
* resolve all lint issues from multi-select filter implementation ([3a3653a](https://github.com/SentioLabs/arc/commit/3a3653aa4566198b285908e39d6a8281b69d86b8))
* resolve all lint issues from plan simplification ([3506206](https://github.com/SentioLabs/arc/commit/3506206dff843b9cf239bd4d82e67da10774bad4))
* resolve all lint issues to achieve zero-error lint pass ([cfd608a](https://github.com/SentioLabs/arc/commit/cfd608a30edccf26e73514ba541bec4c40687f0d))
* resolve lint issues in AI session code ([16dc8e6](https://github.com/SentioLabs/arc/commit/16dc8e6a8bd7924d1a02be830e46fea92a6b754c))
* resolve lint issues in label CLI and client code ([bfef67a](https://github.com/SentioLabs/arc/commit/bfef67a40e0f53d6f8d8f60643f1866ee68fb9fb))
* resolve lint issues in templates.go and search.go ([6f00b3d](https://github.com/SentioLabs/arc/commit/6f00b3de7e1913d957506b0182d884b5ae322e3c))
* resolve lint issues, migration FK/FTS errors, and add missing getIssue endpoint ([6ce3c7e](https://github.com/SentioLabs/arc/commit/6ce3c7e909bf5d1c5bad918e3706b177e6820136))
* resolve plan file paths to absolute for cross-process access ([27d86e6](https://github.com/SentioLabs/arc/commit/27d86e6a29893b42919ff2140ffde50cb466ea33))
* resolve svelte-check errors, add to lint pipeline ([ecb4b5a](https://github.com/SentioLabs/arc/commit/ecb4b5a0cae2c5247953125a6bf321140c572137))
* show planner URL after plan create ([9db436b](https://github.com/SentioLabs/arc/commit/9db436bb7d511eacd2fabd0a951ee6cbce9d79e3))
* **storage:** enforce foreign keys and clean orphaned workspaces ([bf16cb8](https://github.com/SentioLabs/arc/commit/bf16cb8aeb0ad1700f621427925c127c33197f78))
* **storage:** use dynamic SQL for multi-value filters (sqlc.slice + sqlc.narg incompatible) ([94ef1e0](https://github.com/SentioLabs/arc/commit/94ef1e014ab5fee655f98852c88f87639d948d47))
* suppress all session-start errors in hook mode ([89136d3](https://github.com/SentioLabs/arc/commit/89136d3f297d2bd41bc7271c1dd5edad9cfaa9d9))
* **tests:** make plan test temp dirs world-traversable for Docker ([4a254d9](https://github.com/SentioLabs/arc/commit/4a254d9737380b5316061f49ac4056de75507212))
* **tests:** run test server container as host UID ([1adc400](https://github.com/SentioLabs/arc/commit/1adc400377ac3848c57adae30df348ca6c597f29))
* **tests:** use 127.0.0.1 instead of localhost in containers ([fda4d03](https://github.com/SentioLabs/arc/commit/fda4d031d8e995b7101e80a4c8583e253182cf81))
* **tests:** use ARC_SERVER env var from container, not test code ([7c368d1](https://github.com/SentioLabs/arc/commit/7c368d1f19adad59013aa382e88c58b229ed1bc6))
* update migration, tests, and mocks for new plan interface ([c1bf0f3](https://github.com/SentioLabs/arc/commit/c1bf0f355f91fa04eb953c24dfd68e1a85e2df30))
* update nolint directives for golangci-lint 2.11.x ([e4f5822](https://github.com/SentioLabs/arc/commit/e4f58224d39dea2ac001080ca9c87f4da72d4cf6))
* use sentinel error for hook-mode session skip vs real errors ([ab89a4d](https://github.com/SentioLabs/arc/commit/ab89a4d4270a9d903c2ce2a01b2213abb38cca14))
* **web:** associate label with textarea for a11y compliance ([b389f42](https://github.com/SentioLabs/arc/commit/b389f42552bb4a9969aacae49e861c5b375e5cf7))
* **web:** handle unsupported shiki languages, simplify planner UI ([65fc28f](https://github.com/SentioLabs/arc/commit/65fc28f5f6461cbf199c36fc508a20659d7db5dc))
* **web:** redesign transcript viewer and filter progress entries ([12a3584](https://github.com/SentioLabs/arc/commit/12a358456e61ffe5d6d0a2694d0b88ba33b9d727))
* **web:** remove unassigned references from teams page ([99fa704](https://github.com/SentioLabs/arc/commit/99fa704c2fff28a25c703c7c70b211a6694e15ff))
* **web:** replace 'as any' with proper UpdatePlanContentRequest type ([3f676cb](https://github.com/SentioLabs/arc/commit/3f676cb34afcd9e2e93a5358e6ba64445b211aa5))
* **web:** replace nested button with span in MultiSelect chips ([2b81b8a](https://github.com/SentioLabs/arc/commit/2b81b8a759941005c90fd32418d151e89bc4b9ec))


### Refactoring

* **api:** rename workspace references to project in handler files ([6856a62](https://github.com/SentioLabs/arc/commit/6856a62cb9588a7100ef7d6552f9e46f9528b678))
* **api:** rename workspace references to project in server.go helpers ([ba3109b](https://github.com/SentioLabs/arc/commit/ba3109bf1cead79c78d7600f9a6c815ff2d7ae61))
* **api:** rename workspace references to project in test files ([10f3236](https://github.com/SentioLabs/arc/commit/10f32363dd3fbf4ba7c689486d597b291c8bed06))
* **api:** rename wsID→pID in createTestIssue helper param ([90971a9](https://github.com/SentioLabs/arc/commit/90971a9d28aaa9e3725de67f5d5c62f3f99447e4))
* **tests:** fully self-contained integration tests in Docker ([f81cb59](https://github.com/SentioLabs/arc/commit/f81cb590860706aa036feb6c0678219ea938b315))
* **tests:** self-contained integration tests in Docker ([413f15a](https://github.com/SentioLabs/arc/commit/413f15a937a662618ea6d1018fe7c69abe468136))
* unified plan storage with approve/reject workflow ([97df0fb](https://github.com/SentioLabs/arc/commit/97df0fbc3fa3cecaf5fede85dd714f3891e44d13))
* **web:** use shared API client in project plans page ([dc5f1d5](https://github.com/SentioLabs/arc/commit/dc5f1d50d4f16e566d400c91e3b3ea76c7345cea))

## [0.12.0](https://github.com/SentioLabs/arc/compare/v0.11.2...v0.12.0) (2026-03-13)


### Features

* add arc db backup command with semver-aware pre-update backups ([23a71e5](https://github.com/SentioLabs/arc/commit/23a71e54f9a5aac56b940129459211de38d963bf))
* add workspace merge CLI command and git worktree detection ([7bbd0c7](https://github.com/SentioLabs/arc/commit/7bbd0c7973e82d6639ac0071ad32a05e594ec399))
* **api:** add filesystem browse endpoint for web UI path picker ([71ea3d7](https://github.com/SentioLabs/arc/commit/71ea3d7fec81a305e3ad89dd255c46b4b78f1017))
* **api:** add workspace merge endpoint ([23ebb2e](https://github.com/SentioLabs/arc/commit/23ebb2e2290068f78767c5ed6b0fcb504e3e8acb))
* **api:** add workspace paths REST endpoints ([035e1bd](https://github.com/SentioLabs/arc/commit/035e1bd7103ee6df9a09f1c4f0b1199865beeac4))
* **ci:** add GitHub Actions test workflow ([bdd5d5b](https://github.com/SentioLabs/arc/commit/bdd5d5b6519751e588bfd23ec3a043793508c008))
* **cli:** add arc migrate-paths for one-time path migration ([cda98ba](https://github.com/SentioLabs/arc/commit/cda98ba4597f38acc4c7046f92ab9679e58b9deb))
* **cli:** add arc paths command for path management ([73ed6f9](https://github.com/SentioLabs/arc/commit/73ed6f9320d8ef7823f4c076fa685466abd72449))
* **cli:** add workspace merge command ([760a5f0](https://github.com/SentioLabs/arc/commit/760a5f0abd7113c6524ab1cc99e864187d24e1da))
* **cli:** add worktree auto-detection to workspace resolution ([af6156f](https://github.com/SentioLabs/arc/commit/af6156fd62223dcab23512091eb24b25dfceff9b))
* **cli:** color-code priority badges instead of ambiguous filled/empty dots ([2e600d3](https://github.com/SentioLabs/arc/commit/2e600d39c79fdd559108a356b40742b83572ad29))
* **client:** add workspace paths client methods ([a4ea0a5](https://github.com/SentioLabs/arc/commit/a4ea0a5c246120c50fdb62ed95a424c52ded4955))
* **cli:** support name-based workspace lookup in arc init ([cef2103](https://github.com/SentioLabs/arc/commit/cef2103bd6d9c1874cfaf30b527c35c2ee144e53))
* **cli:** use server-side path registration and resolution ([1289cd1](https://github.com/SentioLabs/arc/commit/1289cd142d54903d72cab69836eaa97f42f2cf4e))
* **plugin:** add eval suite and optimize skill descriptions ([5dc5d9e](https://github.com/SentioLabs/arc/commit/5dc5d9ec2c7d2c22e727b2d4283da2a583cea40a))
* **plugin:** add gate check phase to arc-implementer agent ([3e9d776](https://github.com/SentioLabs/arc/commit/3e9d776ec5361cbb35c6b88e61d3dcf61add409c))
* **plugin:** add integration test checkpoint to implement skill ([24c843b](https://github.com/SentioLabs/arc/commit/24c843b0a147eb8c29c5ad1da578e01de3457301))
* **plugin:** add quality gate to arc-implementer agent ([29db89e](https://github.com/SentioLabs/arc/commit/29db89e37e649ead0aff82b24bc9c6413492792e))
* project rename, dashboard card redesign, and list view two-line layout ([ed9c18b](https://github.com/SentioLabs/arc/commit/ed9c18b006ec0e6665789978d2d26dbe37a0738c))
* **project:** add DetectWorktreeMainRepo for git worktree detection ([6b1f593](https://github.com/SentioLabs/arc/commit/6b1f59363e29b89556f413cf370a9ca13c4cd459))
* **storage:** add workspace merge with transaction safety ([4420cd5](https://github.com/SentioLabs/arc/commit/4420cd5c3832802304995ada05a484b0ea0c0321))
* **storage:** implement workspace paths SQLite storage ([d77563c](https://github.com/SentioLabs/arc/commit/d77563c43aa91afab485783c97a1173f908d84c8))
* symlink-aware path resolution with three-tier workspace lookup ([2908262](https://github.com/SentioLabs/arc/commit/290826244c7a929dacbe391c9f37927d3f6e8823))
* **test:** add docker compose test profile and orchestration script ([e68b19e](https://github.com/SentioLabs/arc/commit/e68b19ee04861c76622d512e6aff131cb7a969c9))
* **test:** add Go integration test helpers and initial CLI tests ([e90ff9f](https://github.com/SentioLabs/arc/commit/e90ff9fe5ca044ae0d958e2116447d2d28da2866))
* **test:** add Playwright E2E config and smoke test ([c99b97b](https://github.com/SentioLabs/arc/commit/c99b97b2f73e81d9a70f40385e81d487762146f4))
* **web:** add filesystem browser for path selection ([77996fc](https://github.com/SentioLabs/arc/commit/77996fc4a143cdb7104ac14cdb23059d185b5921))
* **web:** add workspace merge UI ([3500c79](https://github.com/SentioLabs/arc/commit/3500c79250a4758e99789cf2d7c42b3a97f86217))
* **web:** add workspace paths display and management to workspace cards ([2bcc15b](https://github.com/SentioLabs/arc/commit/2bcc15b15deb6c563f8ee6a5a9901f378efccafc))
* **web:** add workspace paths reporting dashboard ([b223b55](https://github.com/SentioLabs/arc/commit/b223b552d95e076ce06b2238edcd76c5c7e92bd2))
* **web:** redesign workspace detail with paths section and actions menu ([77cff68](https://github.com/SentioLabs/arc/commit/77cff683df2f0bff4fa054f66a3a93360665161a))
* **workspace-paths:** add path_type column to distinguish symlinks from canonical paths ([3db00f8](https://github.com/SentioLabs/arc/commit/3db00f81acb4dc031522fc5f44ec1da3f4a8c5b9))
* **workspace-paths:** add shared contracts, migration, and code generation for multi-directory workspaces ([fc79b5c](https://github.com/SentioLabs/arc/commit/fc79b5c331c3a12a9aa91c765b0bcd03a7ea2ba9))


### Bug Fixes

* address code review findings from CLAUDE_REVIEW.md ([6e5b3e3](https://github.com/SentioLabs/arc/commit/6e5b3e35aac33810ea0e3031be21a39e44520a08))
* **ci:** bump golangci-lint to v2.11.2 to match local version ([c4720e9](https://github.com/SentioLabs/arc/commit/c4720e96807a3104f9faa10ff811740ddd5b7d94))
* **ci:** upgrade golangci-lint-action to v7 for v2.x support ([c1a8ee7](https://github.com/SentioLabs/arc/commit/c1a8ee7e68f2affaa35744a1b046c7d62a2aaecd))
* **ci:** upgrade setup-go to v6 and golangci-lint to v2.1 with goinstall ([a7c0627](https://github.com/SentioLabs/arc/commit/a7c0627936b9fda2dfa5363d56bc6faaf0bd7b9d))
* **ci:** use golangci-lint v2.10.1 binary instead of goinstall ([8e1db07](https://github.com/SentioLabs/arc/commit/8e1db071d46ad24734ad4d557e5dadba48f392db))
* **cli:** normalize symlinks and add server fallback for workspace resolution ([e6104e1](https://github.com/SentioLabs/arc/commit/e6104e101e077fea01830fe06903c303dbb4ed46))
* correct path_type on existing paths and align manage paths UI ([2bbff27](https://github.com/SentioLabs/arc/commit/2bbff27557dbda7daf70d66701c9b95d56f9e093))
* **e2e:** add sync points to workspace CRUD tests to prevent flaky failures ([a61644f](https://github.com/SentioLabs/arc/commit/a61644f940478cc7be89e5848a7f4df38c8f31dd))
* **e2e:** fix labels tests to use shared fixtures API ([0f4491c](https://github.com/SentioLabs/arc/commit/0f4491c357d915239ef2d6a7d202f9e71df191cc))
* **e2e:** fix Playwright test infrastructure ([af1cf96](https://github.com/SentioLabs/arc/commit/af1cf9622996d5094ff3e8f662c3a9d3163c44d2))
* **integration:** fix test isolation and assertion issues ([94960ea](https://github.com/SentioLabs/arc/commit/94960ea6693f8a46c1e5a5c94f2f46406b564bb1))
* **lint:** resolve all golangci-lint warnings ([2e10639](https://github.com/SentioLabs/arc/commit/2e106390002c28b99d6a1855b0577ac8a8744149))
* remove deprecated project references and fix test imports ([e92279b](https://github.com/SentioLabs/arc/commit/e92279bf50f835fe868d060ee3db664f2bc4c606))
* remove language key ([6488d11](https://github.com/SentioLabs/arc/commit/6488d112ead4ffaba00194ce9be2644f67158050))
* remove leftover project.CleanupWorkspaceConfigs reference in merge handler ([6126ab3](https://github.com/SentioLabs/arc/commit/6126ab313918b8d4769d8c48e2c7d687c0a473db))
* remove old workspace_ops_test.go (renamed to project_ops_test.go) ([46c6341](https://github.com/SentioLabs/arc/commit/46c63417bac6987f4cade9af2168026b6e38be54))
* resolve all golangci-lint errors across codebase ([852efbb](https://github.com/SentioLabs/arc/commit/852efbb15ba404f48e3a7693814caf3355c645c5))
* resolve flag collision, add worktree detection, update docs ([3aa39fe](https://github.com/SentioLabs/arc/commit/3aa39fe825d0750d192a42357e3ce64f0566be76))
* **web:** resolve all check and lint errors ([cc88a5c](https://github.com/SentioLabs/arc/commit/cc88a5c1efcf2d89f2241a526351646decbd9099))
* **workspace:** normalize cwd in resolveWorkspace to handle symlinks ([ce1970e](https://github.com/SentioLabs/arc/commit/ce1970ef58f00a9ef86d99857c0e1f4d6d8896ff))


### Refactoring

* **api:** complete workspace→project rename in API layer (T2) ([7f8dfad](https://github.com/SentioLabs/arc/commit/7f8dfad6e93d9a0a6d9f23f692a74f0971e6d017))
* **api:** rename routes from /workspaces/ to /projects/ ([776fd18](https://github.com/SentioLabs/arc/commit/776fd18e444e066f4f19ecac3f38620483c92110))
* **cli:** complete workspace→project rename in CLI layer (T4) ([eee5e5f](https://github.com/SentioLabs/arc/commit/eee5e5fa62ef7624ee3a19af71f52d046ef277c8))
* **client:** rename methods and routes workspace→project ([034d647](https://github.com/SentioLabs/arc/commit/034d647d3a37c85c358b0a7351436eb279c9230a))
* **cli:** rename commands and flags workspace→project ([c168f89](https://github.com/SentioLabs/arc/commit/c168f89f9c6f4590125b5f0abae24a7169e6fb97))
* code review cleanup for workspace paths feature ([3c33f98](https://github.com/SentioLabs/arc/commit/3c33f980a1896e1e69e285ec27d2d53560f511fe))
* rename workspace-&gt;project types, interface, and schema (T0 foundation) ([c359ca2](https://github.com/SentioLabs/arc/commit/c359ca27d8609d36e49ddbc48c2ae053f2ca93af))
* rename workspace→project in OpenAPI spec and regenerate ([14f6df5](https://github.com/SentioLabs/arc/commit/14f6df547bd4bf3e3a0acf1aaea46db4dfddf6b3))
* **storage:** rename sqlite implementation workspace-&gt;project ([15ef805](https://github.com/SentioLabs/arc/commit/15ef80571320889608820dd6dd96451c9a69273e))
* **tests:** rename workspace to project terminology in integration tests ([0669715](https://github.com/SentioLabs/arc/commit/066971545b4684e744dff1e1115961b88490b214))
* **web:** fix remaining workspace→project label in ConfirmDialog (T5) ([a71a30a](https://github.com/SentioLabs/arc/commit/a71a30a1863c4984201167cf288c7e06095f1f4a))
* **web:** rename workspace to project in SvelteKit frontend ([cd4dc58](https://github.com/SentioLabs/arc/commit/cd4dc584922bc3d144297104f5001ee79563d8c5))
* **web:** rename workspace-&gt;project in SvelteKit frontend ([4c6b576](https://github.com/SentioLabs/arc/commit/4c6b5760bfdee06f4bf900866ea614f55bf46aa4))

## [0.11.2](https://github.com/SentioLabs/arc/compare/v0.11.1...v0.11.2) (2026-03-10)


### Bug Fixes

* **storage:** compose list filters with AND instead of mutually exclusive switch ([43279e1](https://github.com/SentioLabs/arc/commit/43279e17ea97fee21f04ef4af8858476b914dc68))

## [0.11.1](https://github.com/SentioLabs/arc/compare/v0.11.0...v0.11.1) (2026-03-08)


### Bug Fixes

* **skills:** prevent scope creep and merge conflicts in parallel worktree execution ([c16bc71](https://github.com/SentioLabs/arc/commit/c16bc71ae643525c571747fa8220f5ba47a59d90))

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
