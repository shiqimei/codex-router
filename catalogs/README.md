# Provider catalogs

`grok2api.models.json` is based on the local service's `/v1/models?client_version=0.155.0` catalog, with one verified correction:

- Grok 4.7: low, medium, high, xhigh; default medium. On 2026-09-22 actual `/v1/responses` requests completed with each corresponding effort and nonzero reasoning token usage. `none` returned invalid_argument. `xhigh` also completed. Its returned metadata said high; that alone does not establish the effective upstream effort. The user confirmed four-level support, and the catalog preserves xhigh.
- The service's static reasoning capability map omitted Grok 4.7 and fell back to `none`; do not blindly overwrite this correction with that older service catalog.
- Context window values remain those supplied by the service. In particular, its 128000 value for Grok 4.7 is a fallback, not an independently measured upstream limit.

Other provider models/credentials/configuration are not changed by this correction.

Selectable Grok models declare `input_modalities: ["text", "image"]`, as confirmed by the user. Grok 4.7 image understanding was tested through the real native app-server with a synthetic two-color PNG; Kimi K3 passed the same test. Kimi K3/K3-256K already declared text and image in the provider-owned catalog.
