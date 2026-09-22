# Validation

Tested on macOS Apple silicon with official desktop 26.915.31945 (9922), bundled Codex 0.155.0-alpha.9.2, Go 1.27.1. This document distinguishes deterministic protocol coverage, real service coverage, and native desktop observations.

## Automated checks

- Race-enabled Go tests covering inherited account/quota/profile logic and native provider configuration persistence.
- Go vet, JavaScript syntax, Python syntax.
- Native configuration retains arbitrary provider fields (including nested query parameters), shared tool definitions, isolated project trust, private environment files, and restart persistence. Invalid TOML creates no partial account and does not echo credential contents.

## Real Codex engine with deterministic Responses endpoints

`tests/e2e/routing.py` starts the actual bundled Codex app-server processes through the compiled mux. Only the upstream HTTP model service is a fixture.

Verified:

1. Create a thread on provider A; follow up on B with the original marker in the actual model request history.
2. Switch during a running turn; continue on A with a new turn ID; steer using the previous active turn ID; observe the steering text in the target model request.
3. Merge histories from all homes without duplicate thread entries or ownership regression.
4. Restart mux and children; resume and follow up on the same thread ID and owner.
5. Return a model-side quota error after `turn/start` succeeds; automatically continue on another connection.
6. Seed the synthetic source with account-bound encrypted reasoning and encrypted compaction; the real target app-server recalls the plain marker without either opaque payload in its model request, and the source file is unchanged.

## Real subscription and provider traffic

`tests/e2e/live.py` uses synthetic marker prompts in private test homes:

- Primary ChatGPT subscription successfully replies.
- A second signed-in Business workspace returns `usageLimitExceeded` / out of credits. The router automatically moves the thread to a usable account and recalls its original marker.
- Subscription → configured `grok2api` (`grok-4.5`) recalls the original marker.
- Provider → Primary subscription recalls it again.

The Business account's device login is verified. Successful model generation on that depleted account cannot be claimed; it validates the depletion/failover path instead. No reset credits were consumed. Temporary copied credentials were removed after testing.

## Native desktop observations

- Superior runs as its own copied application, profile, bundle identity and loopback service; the official application remains running independently.
- The reference project's native profile-menu components render subscription plan labels, masked emails, pooled and per-account quota, and provider rows.
- Adding a provider through the native dialog succeeds.
- Selecting that provider in the profile menu routes a newly created native desktop thread to it.
- Switching the existing native desktop thread to Primary and sending a follow-up produces `ORCHID_7391`, the original marker, in the actual desktop transcript.
- Switching a running provider turn to Primary continues it. A queued native composer message is delivered with the native **Steer** button; the final desktop response is exactly `STEER_8426`.
- The native Usage sheet switches between Primary (86% in that observation) and the depleted Business subscription (0%). No reset credit was consumed.
- A new regression assertion requires the authoritative model/settings notification after migration.
- A tool-using, compacted native thread exposed an encrypted-content portability failure on Grok. The migration now strips foreign encrypted reasoning and replays the plain history before encrypted compaction; unit coverage verifies that tool call IDs/results and visible messages survive. Re-migrating that same native thread and retrying on real Grok returned `ORCHID_7391` successfully.

## Environmental boundaries

The desktop is ad-hoc signed because this Mac has no usable Apple signing identity. Native Computer Use screenshot/edit/save flows now pass on this Mac with its existing privacy grants (see the regression below). Independent helper identity, fresh-machine permission onboarding and Appshots remain separate, unverified coverage.

Provider support is the native configuration surface of the installed Codex binary, not a claim that every provider service/model/credential combination was tested. Rollout handoff preserves conversation history and permissions; it cannot transfer provider-side caches, pending external-process state, or service-specific OAuth access between different identities.

## Delivered state

The installed mux matches the built artifact byte-for-byte. The synthetic Desktop E2E provider is disabled and its fixture service stopped. Primary, the signed-in Business subscription, and the native Grok provider remain configured; new conversations default to Automatic. One rollback app backup is retained.

## Computer Use regression — 2026-09-22

Both phases passed in the same Superior thread `01a0c85e-93e7-7281-a2af-8ac722333447`:

| Phase | Native CUA calls | Returned screenshots | Saved random marker | Alternate automation |
| --- | ---: | ---: | --- | --- |
| Primary subscription | 3 | 2 | Exact match | None |
| Grok provider after handoff | 3 | 2 | Exact match | None |

The task controlled the disposable TextEdit document through `mcp__cua_repl.js`, captured before/after images, saved with Command-S and verified fresh accessibility output. The verifier checked the authoritative account rollout and actual file contents. Reports are in `build/computer-use-regression/primary-result.json` and `provider-result.json`; historical failed attempts were retained instead of counted as passes.

This regression found and fixed two defects:

- Isolated homes inherited plugin enablement without a usable installed CUA runtime. Plugin caches are now shared without sharing account auth; the active versioned CUA MCP definition is materialized for isolated connections. Disabled marketplace placeholders are not mistaken for the active desktop runtime, and explicit CUA disablement remains respected.
- A provider-generated search item ID exceeded OpenAI's 64-character limit on reverse handoff. Portable history now normalizes long model item IDs deterministically while preserving tool `call_id` joins and thread IDs.

`npm run check` includes eight regression-gate tests plus Go cases for plugin-definition propagation and portable IDs. `npm run test:e2e` remains green. See [the repeatable native runbook](COMPUTER-USE-REGRESSION.md).

## Connection menu refinement — 2026-09-22

Verified in the installed desktop: Add another subscription and Add provider are only inside Manage connections. Account emails display directly without hover masking. Add provider opens from Connections and Cancel returns to Connections. JavaScript/Python syntax checks and desktop build/signature verification passed.

Verified subscription rows now show only the plan (Pro 20x / Business), email and remaining quota; redundant Primary / subscription-number prefixes are removed.

Verified Manage subscriptions appears immediately above Settings with the supplied SVG icon, using the same native menu item layout and aligned icon/text columns.

Verified the menu now starts at Usage remaining, without the duplicate profile identity header or its separator. Provider titles omit the redundant API provider suffix.

Account rows now have 1px spacing between adjacent entries. Grok uses the supplied two-path SVG instead of initials; its rendering was checked in the native subscription management dialog.

## Provider editing and deletion

State tests verify stable provider identity, secret preservation/redaction, explicit environment clearing, stale-editor rejection, validation rollback, protected Primary subscription, durable deletion and Undo. Real app-server E2E tests verify changed endpoint/model on the next follow-up, active-turn mutation rejection, unloaded/ephemeral history handling, failed runtime rollback, deletion clearing the default, continued conversations on an available provider, and restoration.

Native desktop checks exercise the prefilled editor, saving a temporary provider rename, deletion confirmation, removing its row and Undo. Manage subscriptions uses the supplied outlined people icon in the same aligned position above Settings.

## grok2api cumulative tokens

Verified live SQL sum, Superior account API and native menu: `269,528,111` total tokens, displayed as `269M`. This matches the supplied dashboard snapshot. Formatter checks verify truncation to compact units and the full-number tooltip. Go tests cover exact totals without adding cached tokens twice, read-only failure handling and local-instance endpoint binding.

## 2026-09-22 — provider catalogs and native model synchronization

- Added per-provider models.json storage, validation, revision checks and rollback. Scoped native model/list reads the selected provider catalog. Fixture E2E verifies two distinct models, explicit model selection and persisted selection after mux restart.
- Installed the local grok2api Codex catalog (five selectable text models, one hidden video model) into the existing Grok connection, preserving its configured grok-4.7 default and credentials.
- Native computer-use inspection confirmed the five Grok model choices, subscription menu loading with 269M token usage, and switching the new-task composer back to GPT-6 Astra Light.
- Found and fixed an HTTP connection-pool regression: per-picker SSE streams starved account requests. Menu and model hooks now share one ref-counted stream. A regression test mounts twenty subscribers and verifies one stream, event fan-out and final cleanup.
- Renamed installed app to CodexRouter, retaining its existing profile/state and a legacy Superior.app symlink for cached tool paths.
- After preserving the user’s provider form, restarted and verified native new-task Grok 4.7 None; existing synthetic task switched to Grok 4.7 None and back to GPT-6 Astra Light. Custom provider choices no longer persist into the Primary model defaults. The desktop build executes the patched native catalog filter to catch runtime scope errors, including support for none/max reasoning levels.

## 2026-09-22 — Grok 4.7 reasoning catalog correction

The local grok2api static capability map omitted 4.7 and returned none as its unknown-model fallback. Real Responses probes accepted low/medium/high, rejected none, and accepted xhigh while returning high in metadata. Per user confirmation, corrected the owned catalog to low/medium/high/xhigh with medium default. Also normalize stale unsupported effort values in turn/start, thread resume and collaboration settings; preserve valid xhigh. No restart or credential changes were needed.

The account menu summary now counts connected subscriptions and providers separately (for example, `2 subscriptions 2 providers`). Regression coverage repairs unsupported stale effort values while preserving xhigh in both direct and collaboration settings. Full unit/race/vet and real app-server fixture E2E passed.

## Provider usage adapters

Added separate Grok and Kimi adapters behind a shared usage contract. Kimi's real API returned `limit_7d.used_ratio=0.000853` (99.9147% remaining), whereas legacy limit/remaining rounded to 100/100; the adapter prioritizes the precise ratio. Probes confirmed the official coding endpoint requires a client User-Agent. Unit coverage includes precise/legacy weekly schemas, malformed values, origin restrictions, no redirected credential forwarding, source instance binding, and token totals without double counting.

The Grok adapter reads all two account weekly billing snapshots: 4% and 0% used. The user confirmed both accounts are Heavy; equal capacity weights give 98% remaining. Weights can be overridden per account in the existing private metrics binding. Missing/expired weekly snapshots are rejected rather than overestimating the combined balance. Direct adapter probes returned Grok 98% and Kimi 99.9147%.

Final installed desktop AX inspection showed `2 subscriptions 2 providers`, Grok `98%`, and Kimi `100%` (rounded from 99.9147%). The opt-in `test:live-history` copied the exact synthetic ORCHID rollout into an isolated mux, injected a foreign encrypted reasoning item, migrated it to real Grok, and deliberately sent stale `none` in both direct and collaboration settings. The turn completed with the original marker; the source rollout hash was unchanged and copied credentials were removed.

## Connection overview header

Replaced the misleading cross-connection `Usage remaining` total with a compact, muted `2 subscriptions · 2 providers` header. No aggregate percentage or chart icon is shown: each connection retains its own quota badge and measurement semantics. Removed the unused aggregate calculation from the account menu.

The connection-count header was subsequently removed at the user's request. Account rows now start the menu directly. A displayed 0% includes the next known reset as `(resets in N days)` above 24 hours or `(resets in N hours)` otherwise, rounded up with singular/plural handling. Unknown or elapsed reset timestamps do not invent a countdown. This formatter applies to subscriptions and provider weekly quotas; boundary tests pass.

Reset countdown copy uses the user-selected compact form, such as `0% (↻ 3d)` / `70% (↻ 12h)`, for every percentage quota with a known future reset. The duration rules and unknown-reset behavior are unchanged; formatter checks pass.

Verified Grok aggregation selects the minimum future reset timestamp across account snapshots, independently of row order. All quota badges now include the compact countdown, not just exhausted accounts.

Menu quota labels use dot separation (`70% · ↻ 6d`), replacing parentheses. Provider menu subtitles now show only the provider identifier (`grok2api`, `kimi`), without the model suffix. Formatter checks and the pinned desktop build pass.

Management UI now uses the bundled YKi switch and native medium button sizing, theme border/input tokens, and dialog heading typography. Initial dialog focus goes to its heading (without a ring); keyboard navigation still retains native control focus rings. Row dividers explicitly use the native border color. Menu reset text moved into the left subtitle; its account text, dot, and reset text are separate flex items with equal 4px gaps and inherited font metrics. The temporary switch regression was restored to the original enabled state.

## Image-input declarations

Corrected visible Grok model input modalities to text/image. Kimi’s existing K3 and K3-256K declarations were already correct. `test:live-vision` validates native model/list inputModalities and sends a synthetic red/blue PNG through each real provider in isolated ephemeral tasks. Grok 4.7 and Kimi K3 both returned LEFT=red, RIGHT=blue; copied credentials were removed. The running Grok desktop task also accepted a user image after the catalog refresh, with no restart.
