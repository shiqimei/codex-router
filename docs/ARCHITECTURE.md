# Superior routing lifecycle

The original transport, subscription auth, quota scoring, control API, and native menu are adapted from the MIT upstream. The official Codex executable remains the agent runtime and model protocol implementation.

## Connection profiles

`state.Account` represents either a ChatGPT subscription or a native Codex provider. Subscription homes force file-backed credential stores and strip inherited forced-login workspace restrictions. Provider homes layer the user's full native TOML over shared tool configuration and inject private environment values at process start. A complete config is published with one atomic rename. Provider secrets are not part of public account snapshots. The shared plugin cache exposes installed definitions without sharing `auth.json` or the private plugin app-server directory. Since plugin installation discovery is account-dependent, isolated connections also receive the desktop-enabled CUA MCP definition from the active versioned cache, preserving explicit disablement and all native client permission checks.

## Thread ownership and handoff

Each thread has a durable owner. Per-thread mutexes serialize requests and switches; the independent notification loop can still receive completion while handoff waits for interruption. Handoff performs read → interrupt if active → await completion → unsubscribe original writer → copy committed rollout → target resume → persist owner → publish native settings → continue when necessary.

Current app-server versions require the copied rollout to reside under the target home for discovery, even when `thread/resume.path` is supplied. The filename and thread UUID are preserved. Provider-generated model item IDs longer than 64 characters are deterministically shortened, retaining their prefix; tool `call_id` joins are unchanged. The copied history drops opaque reasoning and item references. An encrypted compaction record is skipped rather than retaining its unusable blob, allowing the target to replay preceding plain response items and tool results. Plain compaction summaries and visible event logs remain. This avoids forwarding OpenAI account-bound `encrypted_content` to an incompatible provider. Rollouts retain their original date-derived directory so future switches overwrite the target's earlier snapshot instead of creating conflicting copies.

Before ownership commits, errors restore the source backend. After commit, a continuation failure is reported as such; it is not described as a successful response. Incoming notifications from the obsolete owner are dropped. Thread listing only includes the committed owner's copy.

An active handoff creates a new turn. Prior turn IDs map to the replacement only for that thread's current handoff chain. Unrelated stale IDs retain the app-server's normal precondition error. Completion clears aliases. This is a controlled interrupt/resume, not transfer of an in-flight network request or external command process.

## Failure recovery

Synchronous `turn/start` quota failures and asynchronous failed-turn quota events use the same handoff path. A per-turn exclusion set prevents cycling through exhausted profiles, and a short cooldown prevents immediately selecting them for a fresh conversation. Other failures are surfaced without automatically replaying tools.

## Desktop integration

The native menu is inserted at exact anchors in the currently supported renderer. Original React, menu item, avatar layout, modal, and usage-sheet primitives are reused. No separate floating routing panel is installed. The native launch wrapper, Electron profile override, independent URL scheme, ASAR integrity update, and re-signing produce a separate app without changing the official installation.

## Provider lifecycle

Provider edits carry a revision over the private config/environment and label. Blank environment updates preserve secrets; the read API returns saved environment key names rather than values. Edits serialize against thread requests, reject active providers, atomically replace native configuration and restart that provider's child. Owned idle threads resume lazily on their next turn. Invalid runtime configuration restores the old profile and process.

Deletion is a recoverable tombstone: it disables routing, clears a matching default, and hides the provider from account lists without deleting rollouts. A follow-up to an owned historical thread can migrate to an available connection. Undo restores the previous enabled state without changing an unrelated default. Running turns block deletion; unloaded or ephemeral historical entries do not.
