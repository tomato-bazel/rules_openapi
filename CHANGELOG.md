# Changelog

All notable changes to rules_openapi. The format is loosely
[Keep a Changelog](https://keepachangelog.com/) — version headers
mirror the published bazel-registry entries.

## 0.4.0 — Rust path is opt-in (Go consumers no longer pull rules_rust)

- **Breaking (Rust consumers only):** `rules_rust`, `rules_jsonschema`,
  `crate_universe`, and the Rust toolchain are now `dev_dependency` wiring, and
  the Rust codegen toolchain is registered in this repo's `.bazelrc` rather than
  in `MODULE.bazel`. `register_toolchains()` in `MODULE.bazel` propagates to every
  downstream module, so a Go-only consumer of `openapi_go_client` was forced to
  pull `rules_rust` and register Rust toolchains it never used. After 0.4.0, a Go
  consumer's module graph is Rust-free.
- Consumers of `openapi_rust_client` now add `rules_rust` + `rules_jsonschema`,
  the `crate_universe` deps, and `register_toolchains("@rust_toolchains//:all",
  "@rules_openapi//rust:default_rust_client_codegen_toolchain")` in their own
  `MODULE.bazel` — see the README "Install" section. No change to the rule APIs
  or generated output.

## 0.3.0 — Go client codegen

- `openapi_go_client` (`//go:defs.bzl`): OpenAPI → typed Go HTTP client
  (`Client` + `ClientWithResponses` + `components/schemas` types), backed by a
  new default toolchain `//tools/openapi_to_go_client` that wraps
  [`oapi-codegen`](https://github.com/oapi-codegen/oapi-codegen). Registered on
  the reserved `go_client_codegen_toolchain_type`; swappable like the Rust one.
- The Go plugin normalizes OpenAPI 3.1 constructs oapi-codegen doesn't handle
  natively so real specs generate cleanly: nullable-type arrays
  (`type: [X, "null"]`), null-only `oneOf` branches, and discriminated `oneOf`s
  with inline branches (lifted to named schemas + a filled-in `mapping`, giving
  proper discriminated unions). It also disambiguates Go field-name collisions
  from dual camelCase/snake_case properties via `x-go-name`.
- The plugin prunes oapi-codegen's broad import block itself with `go/ast`
  instead of shelling out to `goimports`, so it runs in a hermetic sandbox with
  no `go` on `PATH`.
- Verified end-to-end against the WorkOS OpenAPI 3.1 spec (~57k lines of
  generated client, compiles), plus the keeper smoke example + the shared plugin
  contract test.

## 0.2.1 — threadable chrono / uuid / bytes

- `openapi_rust_client` gains `chrono`, `uuid`, `bytes` attrs (default
  to `@openapi_crates`, so existing callers are unchanged). Previously
  these three were hard-coded to `@openapi_crates` while the rest of
  the runtime deps were threadable — a consumer with its own
  crates_universe got a trait-identity mismatch (chrono's serde impls
  resolved against `@openapi_crates`' serde, not the consumer's).
  Threading all runtime deps from one universe fixes it.

## 0.2.0 — docs + CI infrastructure

- Stardoc-generated reference docs for the 4 public-API .bzl files
  in `docs/`: `defs.md`, `providers.md`, `toolchains.md`,
  `contract_test.md`. `bazel run //docs:update` regenerates;
  `bazel test //docs:all` gates the committed copies via
  `diff_test`.
- GitHub Actions CI: `bazel test //...` on ubuntu + macos, plus a
  buildifier lint job.
- `CHANGELOG.md` (this file).
- `.gitignore`: `.claude/` and `MODULE.bazel.lock`.

No API changes.

## 0.1.0 — initial release

- OpenAPI 3 → typed code via the plugin contract from
  `rules_jsonschema` (stdin = OpenAPI doc bytes, argv =
  `--key=value`, stdout = generated source). Identical shape to
  rules_jsonschema, with OpenAPI-specific argv knobs.
- `openapi_rust_client` rule: generates a `rust_library` that
  exports a `Client` struct with one method per OpenAPI operation,
  plus a `types` module containing serde structs for
  `components/schemas`.
- Default Rust client plugin (`tools/openapi_to_rust_client`)
  wraps `progenitor` 0.14.
- `OpenapiCodegenToolchainInfo` provider + per-language toolchain
  types (`rust_client_codegen_toolchain_type` today, more to
  follow).
- `openapi_plugin_contract_test` conformance driver mirroring
  rules_jsonschema's pattern (`valid_minimal`, `malformed_input`,
  `unknown_flag`, `determinism`).
- End-to-end smoke fixture using Oxide's `keeper.json` from
  progenitor's test suite (canonical petstore.yaml has multi-
  content-type request bodies that progenitor doesn't model).
