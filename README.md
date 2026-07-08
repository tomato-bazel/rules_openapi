# rules_openapi

Bazel rules that turn an OpenAPI 3 document into typed code, layered
on top of [`rules_jsonschema`](https://github.com/fastverk/rules_jsonschema)'s
plugin contract. The spec is the single source of truth: regenerated
on every build, plugins live in their target language, swappable via
Bazel toolchains.

## Status: Rust + Go clients

Ships **two paths** today: OpenAPI → typed Rust HTTP client via
[`progenitor`](https://crates.io/crates/progenitor), and OpenAPI → typed Go HTTP
client via [`oapi-codegen`](https://github.com/oapi-codegen/oapi-codegen). Both
sit behind the same language-neutral plugin contract, swappable via Bazel
toolchains. Server stubs and richer composition with rules_jsonschema for
`components/schemas` are the next steps.

## Architecture

Same shape as rules_jsonschema:

```
//openapi:                      language-neutral core
  - toolchain_type per (language, use_case) pair
  - OpenapiCodegenToolchainInfo provider
  - openapi_codegen_toolchain rule (register a plugin)
  - openapi_plugin_contract_test rule (verify conformance)
  - plugin_contract.md (authoritative spec)

//rust:                         Rust output
  - openapi_rust_client rule
  - default toolchain → //tools/openapi_to_rust_client (progenitor-backed)

//go:                           Go output
  - openapi_go_client rule
  - default toolchain → //tools/openapi_to_go_client (oapi-codegen-backed)

//tools/openapi_to_rust_client  default Rust client plugin
//tools/openapi_to_go_client    default Go client plugin
//tools/contract_test           contract conformance driver
//openapi/private/extensions.bzl  http_file pin for the smoke fixture
```

The plugin contract is **identical in shape** to rules_jsonschema's —
stdin = spec bytes, argv = `--key=value`, stdout = generated code,
stderr + exit code for errors. The only difference is what gets
shipped on stdin (an OpenAPI document instead of a JSON Schema)
and the per-plugin flags. See
[`openapi/plugin_contract.md`](openapi/plugin_contract.md).

## Install

`.bazelrc`:

```
common --registry=https://raw.githubusercontent.com/fastverk/bazel-registry/main/
common --registry=https://bcr.bazel.build/
```

`MODULE.bazel`:

```python
bazel_dep(name = "rules_openapi", version = "0.1.0")
```

`rules_jsonschema`, `rules_rust`, a Rust toolchain (1.88+), and
`crates_universe` are pulled in transitively. The default Rust client
toolchain is registered automatically.

## `openapi_rust_client`

```python
load("@rules_openapi//rust:defs.bzl", "openapi_rust_client")

openapi_rust_client(
    name = "petstore_client",
    spec = "//path/to:petstore.yaml",  # or @hosted//file:foo.yaml
)
```

Produces a `rust_library` exporting a `Client` struct with one method
per OpenAPI operation, plus a `types` module containing serde structs
for `components/schemas`. Consumers add it to `deps` and call methods
directly:

```rust
use petstore_client::*;

let client = Client::new("https://api.example.com");
let pet = client.get_pet().pet_id(42).send().await?;
```

### Threading runtime crates

The generated client references `progenitor-client`, `reqwest`,
`serde`, `serde_json`, `regress`, `chrono`, `uuid`, and `bytes` from
its surrounding module scope (progenitor emits trait impls
referencing them unconditionally). Defaults point at
rules_openapi's own `@openapi_crates`; downstream consumers using
their own `crates_universe` instance must thread their crate labels
through to avoid the trait-identity mismatch that
[rules_jsonschema documents](https://github.com/fastverk/rules_jsonschema#two-crates_universe-instances):

```python
openapi_rust_client(
    name = "my_client",
    spec = "spec.yaml",
    progenitor_client = "@my_crates//:progenitor-client",
    reqwest           = "@my_crates//:reqwest",
    serde             = "@my_crates//:serde",
    serde_json        = "@my_crates//:serde_json",
    regress           = "@my_crates//:regress",
)
```

## `openapi_go_client`

```python
load("@rules_openapi//go:defs.bzl", "openapi_go_client")

openapi_go_client(
    name = "petstore_client",
    spec = "//path/to:petstore.yaml",  # or @hosted//file:foo.yaml
    package = "petstore",
)
```

Produces a `go_library` exporting a `Client` + `ClientWithResponses` with one
method per OpenAPI operation, plus Go types for `components/schemas`. Consumers
add it to `deps` (or `embed`) and call methods directly:

```go
client, _ := petstore.NewClient("https://api.example.com")
resp, _ := client.GetPet(ctx, 42)
```

### Threading the runtime

The generated client references `github.com/oapi-codegen/runtime` (parameter
binding) and `.../runtime/types` (Date/UUID/File formats). Defaults point at
rules_openapi's own go_deps; consumers building against their own `go_deps`
universe thread their labels through:

```python
openapi_go_client(
    name = "my_client",
    spec = "spec.yaml",
    package = "myclient",
    runtime       = "@my_go_deps//github.com/oapi-codegen/runtime",
    runtime_types = "@my_go_deps//github.com/oapi-codegen/runtime/types:types",
)
```

### OpenAPI 3.1 normalization

The default Go plugin runs a spec-normalization pass before oapi-codegen for
constructs it doesn't handle natively — so real-world 3.1 specs generate cleanly
rather than erroring:

- `type: [X, "null"]` (3.1 nullable arrays) → `type: X` + `nullable: true`.
- `oneOf` with a bare `{type: "null"}` branch → drop the branch, mark nullable.
- a `oneOf` + `discriminator` with **inline** branches (no `$ref`, no `mapping`) →
  each branch is lifted to a named `components/schemas` entry keyed by its
  discriminator value, and the `mapping` is filled in, so oapi-codegen emits a
  proper discriminated union with named branch types.
- Go field-name collisions from dual camelCase/snake_case properties (e.g. a
  deprecated `createdAt` alongside `created_at`) → an `x-go-name` override on the
  non-canonical property, keeping both.

The plugin also prunes oapi-codegen's broad import block itself (via `go/ast`)
rather than shelling out to `goimports`, so it runs in a hermetic sandbox with no
`go` on `PATH`.

## progenitor's limitations

The default plugin wraps progenitor 0.14, which doesn't handle some
common OpenAPI patterns:

- **Distinct success vs. error response schemas** (e.g. `200` returns
  one schema, `default` returns an `Error` struct). progenitor
  asserts `response_types.len() <= 1`; OAI's canonical petstore
  trips it.
- **Multi-content-type request bodies** (e.g. `application/json` +
  `application/xml`). Swagger's v3 petstore trips this.

Two clean escape hatches:

1. Preprocess the spec — drop the `default` response or the
   alternative content types — before feeding it through the rule.
2. Register your own toolchain pointing at a different codegen
   (`openapi-generator`, hand-rolled, etc.). The plugin contract
   makes that swap mechanical:
   ```python
   load("@rules_openapi//openapi:toolchains.bzl", "openapi_codegen_toolchain")

   openapi_codegen_toolchain(
       name = "my_custom_rust_client_codegen",
       binary = "//path/to:your_binary",
   )

   toolchain(
       name = "my_custom_rust_client_codegen_toolchain",
       toolchain = ":my_custom_rust_client_codegen",
       toolchain_type = "@rules_openapi//openapi:rust_client_codegen_toolchain_type",
   )
   ```

   Then `register_toolchains("//path:my_custom_rust_client_codegen_toolchain")`
   in your `MODULE.bazel` ahead of rules_openapi's default.

## Conformance testing

The `openapi_plugin_contract_test` rule runs the contract scenarios
(valid input, malformed input, unknown flag, determinism) against
any plugin executable:

```python
load("@rules_openapi//openapi:contract_test.bzl",
     "openapi_plugin_contract_test")

openapi_plugin_contract_test(
    name = "my_plugin_conforms",
    plugin = "//my:openapi_rust_client_codegen",
)
```

The default plugin is gated by this test
(`//tools/openapi_to_rust_client:openapi_to_rust_client_conforms`).

## Compatibility

- **Bazel**: 7.4+, bzlmod required (tested on 9.1).
- **Rust**: 1.88+ (transitive deps need stabilised `let-chains`).
- **Go**: SDK 1.24+ (downloaded by rules_go).
- **OpenAPI**: 3.0 + 3.1. The Rust path follows progenitor's coverage (gaps
  documented above); the Go path normalizes the common 3.1 constructs
  oapi-codegen doesn't handle (see `openapi_go_client` above).

## Roadmap

- **v0.3**: Server stubs (`openapi_rust_server`, `openapi_go_server`).
- **v0.4**: Compose with rules_jsonschema — extract `components/schemas`
  in a preprocessing step, pipe through `jsonschema_rust_library`,
  so the client codegen only handles operations. Eliminates the
  type-generation duplication.

## Testing

```sh
bazel test //...
```

| Target | Coverage |
|---|---|
| `//tools/openapi_to_rust_client:openapi_to_rust_client_conforms` | plugin contract conformance |
| `//examples/smoke:keeper_client_test` | OpenAPI → generated client → typed decode round-trip |

## License

MIT.
