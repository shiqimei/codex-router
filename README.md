<div align="center">

# CodexRouter

### Use all your subscriptions in your favorite Codex desktop.

Auto-rotate when the weekly quota is reached — across Codex accounts, and across other model providers. Stay in the same thread.

[![Apple Silicon](https://img.shields.io/badge/Apple_Silicon-arm64-181818?style=flat-square&logo=apple&logoColor=white)](#quick-start)
[![Go](https://img.shields.io/badge/Go-1.26%2B-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev/)
[![Node](https://img.shields.io/badge/Node-22.12%2B-339933?style=flat-square&logo=nodedotjs&logoColor=white)](https://nodejs.org/)
[![Codex desktop](https://img.shields.io/badge/Codex_desktop-26.915.31945-F07818?style=flat-square)](#quick-start)
[![License](https://img.shields.io/github/license/shiqimei/codex-router?style=flat-square&color=181818)](LICENSE)
[![Checks](https://img.shields.io/github/actions/workflow/status/shiqimei/codex-router/check.yml?style=flat-square&label=checks)](https://github.com/shiqimei/codex-router/actions/workflows/check.yml)

**[Quick start](#quick-start)** · **[Demo](assets/CodexRouter.mp4)** · **[How it works](#how-it-works)** · **[Architecture](docs/ARCHITECTURE.md)**

<img src="assets/banner.png" width="960" alt="CodexRouter rotating across Codex accounts when quota runs out" />

<sub>One subscription hits its weekly cap. The same thread continues on another Codex account, or on another model provider. <a href="assets/CodexRouter.mp4">Open the demo</a>.</sub>

</div>

The official desktop signs into one subscription. Hit the weekly cap and the thread stops, even if another Codex account still has quota. CodexRouter is that same Codex UI, as a **separate app on your Mac**. It rotates across the Codex accounts you add, and it also switches the same thread to other model providers — Grok, Kimi, or any native Codex provider — without leaving the app.

## Rotate, don't restart

<table>
<tr>
<td width="50%" valign="top">

### 🔁 Rotate Codex accounts

Every Codex account you add shows up in the profile menu with its own weekly remaining quota. New chats pick the account with the most weekly headroom. This is not random round-robin.

</td>
<td width="50%" valign="top">

### ⏱️ Keep going at the weekly cap

Accounts at 100% on the weekly window or the short window are skipped. A quota, usage-limit, rate-limit, or out-of-credits error rotates to another Codex account and cools the failed one down for five minutes.

</td>
</tr>
<tr>
<td width="50%" valign="top">

### 🧵 Stay in the same thread

Switch mid-conversation. History is copied, the thread id stays, and the turn continues. You steer from the normal composer instead of starting a blank chat.

</td>
<td width="50%" valign="top">

### 🔌 Native providers, same menu

Grok, Kimi, or any native Codex provider can take a turn when that is what still has room. API providers do not get a made-up ChatGPT percentage.

</td>
</tr>
<tr>
<td width="50%" valign="top">

### 📊 Quota where you already look

The bottom-left profile menu shows remaining quota, add account, add or edit a provider, and the connection switch. No second window.

</td>
<td width="50%" valign="top">

### 🛡️ Leave the official app alone

The build copies `/Applications/ChatGPT.app`. It never edits the install you already have, and the copy does not self-update. Do not redistribute OpenAI binaries.

</td>
</tr>
</table>

## Quick start

**macOS · Apple silicon · Go 1.26+ · Node 22.12+ · Python 3.11+ · ChatGPT.app already installed**

One prompt. Point your coding agent at **this repository** and paste:

```text
Adapt the official Codex desktop already installed on this Mac into a separate local app, CodexRouter. Read this repository first and reuse its code. Do not redesign the multiplexer.

Clone or open this repo, then:

1. Build ~/Applications/CodexRouter.app from the local /Applications/ChatGPT.app. Never edit, replace, or redistribute the official install, its ASAR, or any OpenAI binary. Do not commit credentials, ~/.codex, ~/.superior, or provider secrets.

2. Reuse these paths instead of rewriting them: cmd/codex-mux, internal/mux, internal/state, internal/provideradapter, ui/, scripts/build_superior.py. Keep the MIT attribution to b-nnett/codex-subscription-router in NOTICE.md and LICENSE.

3. Support many Codex accounts in one desktop, and rotate between them. Also support native Codex providers. A provider is native Codex config: model, model_provider, and [model_providers.<id>], plus optional environment variables stored privately and injected only into that provider's Codex process. Account rotation is the main path, not an extra.

4. Put connections in the bottom-left profile menu: remaining quota per Codex account, add another Codex account (device-code login), add / edit / delete provider, enable or disable, switch the active connection. API providers must not show a made-up ChatGPT percentage. New chats pick the best enabled Codex account by weekly urgency. Skip accounts already at 100% on the weekly window or the short window. This is not random round-robin.

5. Allow a switch inside an existing thread. Copy the drained history to the target home, keep the thread id, and continue the turn there. If resume fails, restore the source. Do not start a blank chat.

6. Auto-rotate across Codex accounts when quota is exhausted. If a turn fails with a quota, usage-limit, rate-limit, or out-of-credits error, rotate to another enabled Codex account (a native provider is also eligible) and cool the failed account down for 5 minutes. Do not auto-replay arbitrary tool failures. Interrupted shell commands may already have side effects; only conversation state is preserved.

7. Stay on the desktop build this repo pins (26.915.31945 / build 9922, including its ASAR hash and menu anchors) unless you re-verify those anchors. Unknown builds must stop before install. Self-update stays disabled in the copy. Account data under the existing Superior profile must survive a rebuild.

8. Build with:
   npm ci --ignore-scripts && npm run build:desktop
   Then open ~/Applications/CodexRouter.app.

If this checkout is already CodexRouter, install and verify it. Do not invent a second design.
```

Your agent reads this repo, reuses the mux, and patches a **copy** of your ChatGPT app.

1. **Add another Codex account** with device-code login. The menu label is **Add another subscription**.
2. **Add a provider** by pasting native `model` / `model_provider` config. Secrets stay on disk, not in the UI.
3. **Use the profile menu** (bottom-left) to pick a connection, or switch one mid-thread. The demo hits "You've hit your usage limit", then continues with history intact.

<details>
<summary><strong>Build it yourself</strong></summary>

The patch is pinned to desktop **26.915.31945 / build 9922**. A different build stops before install. Quit CodexRouter before replacing a running build. Existing installs are backed up, and account data survives. `Superior.app` remains a compatibility symlink so cached tool paths keep working. The bundle id and `Application Support/Superior` stay as they are on purpose.

```sh
npm ci --ignore-scripts
npm run build:desktop
open "$HOME/Applications/CodexRouter.app"
```

Ad-hoc signing is enough for the routing flows tested on the development machine. `--identity` selects a local certificate. It does not by itself make Computer Use helpers work on a fresh Mac.

```sh
python3 scripts/build_superior.py --identity "Apple Development: Your Name (TEAMID)"
```

</details>

## How it works

```text
Your Mac
┌──────────────────────────┐
│ Codex desktop UI         │
│ profile menu · composer  │
└────────────┬─────────────┘
             │
┌────────────▼─────────────┐
│ codex-mux                │
│ quota · handoff · rotate │
└────────────┬─────────────┘
             │
     ┌───────┼────────┐
     ▼       ▼        ▼
  primary  another   native
  ~/.codex  Codex     provider
            account
```

Routing state is `~/.superior/state.json`. Extra homes and provider config live under `~/.superior/accounts/<id>/codex-home`. Credentials are not returned by the account API. Primary credentials stay in `~/.codex`.

A handoff waits out the active turn, copies a portable rollout into the target home, resumes the same thread id, and commits the new owner. Failure before commit restores the source. Account-bound encrypted reasoning is stripped so it is not forwarded to another provider. A continued turn gets a new turn id; in-flight steering for that handoff is remapped. Only the committed owner is listed.

Explicit quota errors take that same path. A plain tool failure is shown to you and not replayed.

<details>
<summary><strong>Native provider config</strong></summary>

```toml
model = "your-model"
model_provider = "custom"

[model_providers.custom]
name = "Custom"
base_url = "https://your-provider.example/v1"
wire_api = "responses"
env_key = "PROVIDER_API_KEY"
```

Put `{"PROVIDER_API_KEY":"..."}` in the provider's environment field. Desktop apps do not inherit `.zshrc`. Leave the field blank on edit to keep stored secrets, or enter `{}` to clear them. The config is trusted local Codex configuration, including any credential helper it names. Protocol and model support come from **your** installed Codex binary. CodexRouter does not invent a second model protocol.

Each provider can also own a private `models.json` catalog (`model_catalog_json`). `catalogs/grok2api.models.json` is the local grok2api catalog. The configured default model must appear in that catalog.

</details>

<details>
<summary><strong>Quota badges</strong></summary>

- **Codex accounts** — weekly remaining from Codex rate-limit windows. Rotation uses these windows.
- **Grok / local grok2api** — weighted weekly remaining from that instance's account billing snapshots: `sum(weight × remaining%) / sum(weight)`. Same tier defaults to equal weight; optional `accountWeights` can bias capacity. Missing or expired weekly snapshots stay unavailable instead of being dropped. The tooltip adds cumulative tokens (`SUM(total_tokens)` from `request_audits`, via `sqlite3 -readonly`, never double-counting cached input) and the quota sync time. That count is every retained audit row on the service, not only CodexRouter traffic, so a narrower dashboard filter will not match. No admin credentials. Bind it in `~/.superior/provider-metrics.json` as `{"type":"grok2api-sqlite","baseUrl":"http://localhost:8000/v1","databasePath":"/absolute/path/to/backend.db"}`. If the endpoint no longer matches, the local badge goes away. A database that cannot be read shows a dash, not a fake zero.
- **Kimi Code** — official `api.kimi.ai` / `api.kimi.com` coding hosts only. Weekly remaining comes from `/coding/v1/usages` (`usages.limit_7d.used_ratio`, or the legacy weekly limit). The 5-hour window is not substituted for the weekly number. Redirects are rejected.
- **Everyone else** — an API badge. Missing credentials or a bad response means "unavailable", not 0% or 100%.

`internal/provideradapter` is the extension point: `Match` + `Read` → a shared `Usage` value. The mux caches and dedupes; the menu only formats.

</details>

## Build with us

Found a rough edge? [Open an issue](https://github.com/shiqimei/codex-router/issues). Have an improvement? [Send a pull request](https://github.com/shiqimei/codex-router/pulls).

```sh
npm run check       # race-enabled Go tests, vet, JS and Python syntax
npm run test:e2e    # real official app-server, isolated homes, scripted providers
```

<details>
<summary><strong>Live checks</strong></summary>

```sh
npm run test:live   # opt-in, spends real quota on canary chats only
```

`test:live` copies existing local credentials into private temp homes and deletes those copies when it finishes. It never sends your real conversations. Keep tokens out of commits and logs.

What is verified, and what is not: [docs/VALIDATION.md](docs/VALIDATION.md), [docs/COMPUTER-USE-REGRESSION.md](docs/COMPUTER-USE-REGRESSION.md). Design notes: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md). Files in `docs/upstream` are historical reference, not current claims.

</details>

## License

[MIT](LICENSE). The multiplexer, subscription login, and account menu started as MIT code from [b-nnett/codex-subscription-router](https://github.com/b-nnett/codex-subscription-router). Provenance is in [NOTICE.md](NOTICE.md). This repo adds provider profiles, explicit routing, mid-thread handoff, rotation across Codex accounts, and the desktop patch for build 9922.

CodexRouter is not affiliated with OpenAI. ChatGPT, Codex, and OpenAI names say what this copy talks to. You are responsible for the terms of the ChatGPT app on your machine and of every account you connect.

Do not open a pull request that adds API keys, account homes, or a built `CodexRouter.app`.

---

**If CodexRouter keeps your threads moving, give it a star. ⭐**
