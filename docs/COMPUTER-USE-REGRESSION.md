# Native Computer Use regression

This is an opt-in, real desktop test. It runs Computer Use **inside Superior's own task**, first on Primary and then on a configured provider in the same thread. Driving Superior from another app is setup, not evidence that Superior's CUA works.

## Run

1. Prepare a unique run directory:

   ```sh
   npm run test:computer-use -- prepare --run-dir build/cua-run --phase primary
   ```

2. Open the emitted disposable `.txt` target in TextEdit. In Superior, select Primary and submit the emitted prompt in a dedicated test task. The agent must use `mcp__cua_repl.js` to select TextEdit, return a screenshot, edit the target, save with Command-S, and return a fresh screenshot/accessibility state. Keep other documents untouched.
3. When the turn finishes, verify using its actual thread UUID:

   ```sh
   npm run test:computer-use -- verify --run-dir build/cua-run --phase primary --thread-id THREAD_UUID
   ```

4. Prepare the second phase, switch that **same** Superior task to the configured provider, and submit the new emitted prompt:

   ```sh
   npm run test:computer-use -- prepare --run-dir build/cua-run --phase provider
   ```

5. Verify the completed provider phase:

   ```sh
   npm run test:computer-use -- verify --run-dir build/cua-run --phase provider --thread-id THREAD_UUID
   ```

A coding agent can drive steps 2 and 4 using its available Computer Use tool. The verifier itself never modifies the document or supplies a substitute screenshot. Native UI execution is explicit rather than pretending a headless CI run exercised macOS permissions.

## Pass criteria

- Actual native CUA tool namespace in the authoritative rollout for the selected connection.
- Native app selection, screenshot request, native editing and Command-S calls.
- At least two image outputs returned by those native tool calls.
- The saved target exactly matches the phase's fresh random marker.
- Tool output observes the test document or its expected content.
- No shell/filesystem/AppleScript/browser automation fallback.
- No native permission/connection/screenshot error.
- Provider phase uses the same thread whose Primary phase passed, with the owner now a provider.

The verifier writes `primary-result.json` / `provider-result.json`, with checks, trace hash, connection identity and time. It does not copy screenshots, credentials or raw desktop tool output into the report. Only `pass` exits 0; `fail` and `blocked` exit 2. Missing input/state is an error, never an implicit skip.

`npm run check` includes tests of the verifier: a changed file alone, mere tool names in a prompt, ordinary Node REPL, missing screenshots and alternate automation cannot pass. Go tests verify shared plugin definitions remain available in isolated homes without sharing `auth.json` or the private plugin app-server directory.

## Installation and permissions

Preflight records the installed app and helper's signature validity and identifiers. Signature validity alone cannot pass the runtime test. Test on the user's current authorized macOS environment; if Accessibility or Screen Recording consent is missing, stop for user action rather than changing TCC or weakening helper caller checks.

Superior currently uses the bundled, officially signed CUA helper. An ad-hoc desktop signature does not establish independent helper identity or fresh-machine privacy grants. Appshots, other native apps, and initial permission onboarding are separate coverage from this TextEdit read/edit/save test.
