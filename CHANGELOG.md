# Changelog

All notable changes to rules_openapi. The format is loosely
[Keep a Changelog](https://keepachangelog.com/) — version headers
mirror the published bazel-registry entries.

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
