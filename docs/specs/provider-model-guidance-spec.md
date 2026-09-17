# Provider model guidance

`internal/service/provider/model_guidance.go` owns the read-only `guidance` returned
with model records (including discovery and mutation responses) and selectable
model options. Frontend consumers display this projection; they do not infer
capabilities from model names. `category` remains a compatibility field.

## Facts and selection policy

Capabilities are optional booleans: missing means unknown, false means explicitly
unsupported. `text_output`, `vision`, `image_output`, `image_editing`,
`tool_calling`, `reasoning` and `embedding` are independent.

Precedence is user override, persisted provider record, scoped catalog fact,
legacy catalog fallback, then unknown. `sources` distinguishes `user`,
`provider_record`, `catalog` and `legacy_catalog`. Historical provider records may
contain previously materialized legacy guesses; `provider_record` does not claim
that every value came directly from a vendor API. New catalog capability facts are
never written into model records; discovery persists the remote facts only. Clearing an override removes its JSON field.

`model_advice_catalog.go` maintains the Nexus starting-point policy with a catalog
version and explicit provider presets/model aliases. It does not strip namespaces,
infer the vendor of custom endpoints, claim current price/latency rankings, add
undiscovered models, enable models, or replace defaults. The initial chat advice
uses the existing Nexus model catalog; initial image facts correspond to adapter
fixtures in `imagegen/provider_external_test.go` and `imagegen/service_test.go`. These fixtures prove supported
request shapes, not current remote account availability. Review exact IDs and
provider routes and increment the version when updating entries. A recommendation
is emitted only for a locally eligible purpose; unsupported/unrecognized models
remain visible in the management list.

## Purpose eligibility

- Chat: an LLM provider, no embedding capability, and no explicit text denial.
  Unknown ordinary chat IDs remain usable for manual provider compatibility.
  Image output or image/audio/video/rerank/embedding categories require explicit
  text output; a multimodal model may belong to both chat and image selections.
- Vision: chat eligibility plus confirmed image input.
- Generation: confirmed image output and an implemented image protocol. For a
  dedicated custom image provider, explicit endpoint configuration preserves the
  legacy generation declaration when image capability is unknown; explicit false
  still denies it.
- Editing: generation eligibility plus confirmed image editing, excluding the
  ModelScope adapter which does not implement editing. Image input plus image
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

## Presentation

Onboarding, model management, chat menus and general model defaults share entity
presentation. Recommendations sort ahead of peers but never change selection.
Management retains enabled models first. Recommendation and capability labels
are distinct facts; capability overrides offer automatic/supported/not supported.
No recommendation means no badge, not an error or a reason to remove a model.
