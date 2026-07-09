<!-- Generated with Stardoc: http://skydoc.bazel.build -->

Rust user-facing rules for rules_openapi.

`openapi_rust_client` is the Rust client codegen rule:

  1. Resolves the `rust_client_codegen_toolchain_type` toolchain.
  2. Runs the toolchain's binary on the OpenAPI spec (stdin/argv/
     stdout per `//openapi/plugin_contract.md`), producing a `.rs`.
  3. Wraps the `.rs` in a `rust_library` whose deps include
     `progenitor-client`, `reqwest`, `serde`, `serde_json`, and any
     additional crates the consumer threads through.

The default toolchain points at the in-repo `openapi_to_rust_client`
binary, which wraps `progenitor` under the hood. Unlike the Go path,
the Rust backend is dev-gated (so Go-only consumers don't pull
`rules_rust`): a consumer registers it — plus `rules_rust` + the
crate deps — themselves (see the README "Install" section). Swap by
declaring your own `openapi_codegen_toolchain` and registering it
ahead of the default.

<a id="openapi_rust_client"></a>

## openapi_rust_client

<pre>
load("@rules_openapi//rust:defs.bzl", "openapi_rust_client")

openapi_rust_client(<a href="#openapi_rust_client-name">name</a>, <a href="#openapi_rust_client-spec">spec</a>, <a href="#openapi_rust_client-extra_args">extra_args</a>, <a href="#openapi_rust_client-progenitor_client">progenitor_client</a>, <a href="#openapi_rust_client-reqwest">reqwest</a>, <a href="#openapi_rust_client-serde">serde</a>, <a href="#openapi_rust_client-serde_json">serde_json</a>, <a href="#openapi_rust_client-regress">regress</a>,
                    <a href="#openapi_rust_client-chrono">chrono</a>, <a href="#openapi_rust_client-uuid">uuid</a>, <a href="#openapi_rust_client-bytes">bytes</a>, <a href="#openapi_rust_client-visibility">visibility</a>, <a href="#openapi_rust_client-rust_library_kwargs">**rust_library_kwargs</a>)
</pre>

Generate a rust_library of a typed OpenAPI HTTP client.

The library exports a `Client` struct with one method per
OpenAPI operation, plus a `types` module containing serde
structs for `components/schemas`.


**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="openapi_rust_client-name"></a>name |  rust_library target name. Consumers add this to `deps`.   |  none |
| <a id="openapi_rust_client-spec"></a>spec |  label of an OpenAPI `.yaml` / `.yml` / `.json` document.   |  none |
| <a id="openapi_rust_client-extra_args"></a>extra_args |  extra `--key=value` flags passed to the plugin.   |  `None` |
| <a id="openapi_rust_client-progenitor_client"></a>progenitor_client |  label of the `progenitor_client` runtime crate the generated code references. Defaults to `@openapi_crates//:progenitor-client`. Consumers using their own crates_universe must thread this through (and likewise the other runtime-dep attrs below) to avoid the same trait- identity mismatch rules_jsonschema documents.   |  `None` |
| <a id="openapi_rust_client-reqwest"></a>reqwest |  label of `reqwest` (HTTP client the generated code uses).   |  `None` |
| <a id="openapi_rust_client-serde"></a>serde |  label of `serde`.   |  `None` |
| <a id="openapi_rust_client-serde_json"></a>serde_json |  label of `serde_json`.   |  `None` |
| <a id="openapi_rust_client-regress"></a>regress |  label of `regress` (used by typify-generated types nested inside progenitor's output).   |  `None` |
| <a id="openapi_rust_client-chrono"></a>chrono |  label of `chrono` (date-time formats). Must come from the same crates_universe as `serde`.   |  `None` |
| <a id="openapi_rust_client-uuid"></a>uuid |  label of `uuid` (uuid format). Same-universe-as-serde rule.   |  `None` |
| <a id="openapi_rust_client-bytes"></a>bytes |  label of `bytes` (binary format). Same-universe-as-serde rule.   |  `None` |
| <a id="openapi_rust_client-visibility"></a>visibility |  forwarded to rust_library.   |  `None` |
| <a id="openapi_rust_client-rust_library_kwargs"></a>rust_library_kwargs |  forwarded to rust_library (e.g. extra `deps`).   |  none |


