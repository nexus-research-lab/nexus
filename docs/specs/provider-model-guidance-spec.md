# Provider model guidance

`internal/service/provider/model_guidance.go` owns the read-only `guidance` returned
with model records (including discovery and mutation responses) and selectable
model options. Frontend consumers display this projection; they do not infer
capabilities from model names. `category` remains a compatibility field.

## Facts and selection policy

Capabilities are optional booleans: missing means unknown, false means explicitly
unsupported. `text_output`, `vision`, `image_output`, `image_editing`,
`tool_calling`, `reasoning` and `embedding` are independent.

Capabilities first match the exact provider preset and model ID. When that pair has
no entry, an exact model ID can supply capability defaults if all catalog entries
for that ID agree. Provider-specific recommendations and plan notices never follow
this fallback. Versioned provider records then apply, and explicit user overrides
win. Remote false vetoes a catalog positive; a catalog false vetoes an automatic
positive only for the matching provider preset. `sources` identifies `user`,
`provider_record` and `catalog`; there is no
model-family or namespace-stripping capability fallback. A custom endpoint or Azure
deployment may reuse a known ID for a different model, so its provider facts or user
override must correct any name-based default.

Automatic records carry `facts_version: 1` inside their stored JSON. Older records
mixed discovered facts with name-derived vision/reasoning guesses: unversioned
positive vision/reasoning fields are ignored until explicit rediscovery/import;
negative fields and user overrides remain effective. This is a read-time projection,
not destructive database cleanup. Other stored facts and model defaults are preserved.
Token-limit compatibility fallback is separate and is not capability evidence.

`model_advice_catalog.go` maintains exact service/model facts with official source
URLs and a review date. `evidence` exposes these references and a localized notice
key for plan/tier/alias restrictions. `text_only` is emitted only for a catalog entry
explicitly established as text-only and not contradicted by effective capabilities;
missing vision alone never implies text-only. Recommendations are purpose-specific
Nexus policy (`flagship`, `balanced`, `image_generation`, `image_editing`), not vendor
performance measurements or account entitlement checks. Recommendations only apply
to locally eligible, actually listed models; they never add or enable models or
replace saved defaults. Azure deployment names and custom endpoints receive no
provider-specific recommendation from name matching.

See [official evidence and provider coverage](../testing/provider-model-evidence.md)
for the dated research snapshot, endpoints, exact plan distinctions and review gaps.
Catalog changes must update sources/date/version together with regression coverage.

## Purpose eligibility

- Chat: an LLM provider with no explicit text denial. An explicit `text_output=true`
  keeps a model chat-eligible even when the same endpoint also advertises embeddings;
  an embedding-only record with no text evidence remains ineligible. Unknown ordinary
  chat IDs remain usable for manual provider compatibility. Image output or
  image/audio/video/rerank categories require explicit text output; a multimodal model
  may belong to both chat and vision selections.
- Vision: chat eligibility plus confirmed image input.
- Generation: confirmed image output and an implemented image protocol. For a
  dedicated custom image provider, explicit endpoint configuration preserves the
  legacy generation declaration when image capability is unknown; explicit false
  still denies it.
- Editing: generation eligibility plus confirmed image editing, excluding the
  ModelScope adapter which does not implement editing, and Doubao whose native
  image-input request is not implemented by the generic OpenAI multipart edit path. Image input plus image
  output alone does not prove editing.

OpenAI Images, DashScope Image and ModelScope Image are implemented routes.
A Chat Completions/Responses model card alone cannot enable image generation.
Custom providers must configure a supported image endpoint. Existing preset route
resolution is retained. Support for additional multimodal transports is future
work, not implied by a badge.

Provider/model enablement and credentials are checked separately. Runtime text
resolution and image resolution enforce the same admission policy as selectors;
editing additionally checks the resolved editing eligibility before network I/O.
Saved default identities survive catalog updates. An ineligible saved default
fails admission rather than being silently replaced by a recommended model.

## Credentials and defaults

Disabling a Provider preserves its credential, model cards and saved default/Agent
bindings. Re-enabling restores their eligibility. Replacing a key preserves model
configuration; an empty replacement draft leaves the stored credential unchanged.
The explicit clear-key action confirms removal, submits an empty credential and
disables the Provider together. It does not delete the Provider or its models.

Only enabled Providers with a nonempty credential and Base URL enter selectable
options or automatic default resolution. This checks local readiness, not remote
key validity. Without an available Provider, selectors and effective defaults are
empty. The Models page displays unavailable saved selections as empty while keeping
the saved identity so it can be restored when that exact model becomes available.
No recommendation or unrelated available model overwrites the saved selection.

## Presentation

Onboarding, model management, chat menus and the dedicated Models settings page share entity
presentation. Recommendations sort ahead of peers but never change selection.
Management retains enabled models first. Badges distinguish documented text-only
from multimodal image input; recommendation hints explain the intended task and
plan/tier limitations and show the documentation review date. Recommendation and capability labels
are distinct facts; capability overrides offer automatic/supported/not supported.
No recommendation means no badge, not an error or a reason to remove a model.
The Models page groups chat, image generation, vision and background defaults
beside the Providers settings entry. Model management retains the existing
capability icons with accessible names/tooltips and shows recommendation badges
separately.
