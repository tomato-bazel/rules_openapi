<!-- Generated with Stardoc: http://skydoc.bazel.build -->

Go user-facing rules for rules_openapi.

`openapi_go_client` is the Go client codegen rule:

  1. Resolves the `go_client_codegen_toolchain_type` toolchain.
  2. Runs the toolchain's binary on the OpenAPI spec (stdin/argv/
     stdout per `//openapi/plugin_contract.md`), producing a `.go`.
  3. Wraps the `.go` in a `go_library` whose deps include
     `github.com/oapi-codegen/runtime` (the runtime the generated
     client references) plus any additional packages the consumer
     threads through.

The default toolchain (registered by MODULE.bazel) points at the
in-repo `openapi_to_go_client` binary, which wraps `oapi-codegen`
under the hood (with a spec-normalization pass for OpenAPI 3.1
constructs oapi-codegen doesn't handle natively). Swap by declaring
your own `openapi_codegen_toolchain` and registering it ahead of the
default.

<a id="openapi_go_client"></a>

## openapi_go_client

<pre>
load("@rules_openapi//go:defs.bzl", "openapi_go_client")

openapi_go_client(<a href="#openapi_go_client-name">name</a>, <a href="#openapi_go_client-spec">spec</a>, <a href="#openapi_go_client-package">package</a>, <a href="#openapi_go_client-importpath">importpath</a>, <a href="#openapi_go_client-runtime">runtime</a>, <a href="#openapi_go_client-runtime_types">runtime_types</a>, <a href="#openapi_go_client-deps">deps</a>, <a href="#openapi_go_client-visibility">visibility</a>,
                  <a href="#openapi_go_client-go_library_kwargs">**go_library_kwargs</a>)
</pre>

Generate a go_library of a typed OpenAPI HTTP client.

The library exports a `Client` + `ClientWithResponses` with one
method per OpenAPI operation, plus Go types for `components/schemas`.


**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="openapi_go_client-name"></a>name |  go_library target name. Consumers add this to `deps`.   |  none |
| <a id="openapi_go_client-spec"></a>spec |  label of an OpenAPI `.yaml` / `.yml` / `.json` document.   |  none |
| <a id="openapi_go_client-package"></a>package |  Go package name for the generated file. Defaults to a sanitized form of `name`.   |  `None` |
| <a id="openapi_go_client-importpath"></a>importpath |  go_library importpath. Defaults to `package`. Override when consumers import the client by a specific module path.   |  `None` |
| <a id="openapi_go_client-runtime"></a>runtime |  label of `github.com/oapi-codegen/runtime` (the runtime the generated client references for parameter binding). Defaults to `@com_github_oapi_codegen_runtime//:runtime`. Consumers using their own go_deps universe should thread this through so the generated code compiles against the same runtime they build with.   |  `None` |
| <a id="openapi_go_client-runtime_types"></a>runtime_types |  label of `github.com/oapi-codegen/runtime/types` (the Date/UUID/File format types the generated code references when the spec uses those formats). Defaults to `@com_github_oapi_codegen_runtime//types`. Thread from the same universe as `runtime`.   |  `None` |
| <a id="openapi_go_client-deps"></a>deps |  extra deps forwarded to go_library (for specs whose generated code references additional packages).   |  `None` |
| <a id="openapi_go_client-visibility"></a>visibility |  forwarded to go_library.   |  `None` |
| <a id="openapi_go_client-go_library_kwargs"></a>go_library_kwargs |  forwarded to go_library.   |  none |


