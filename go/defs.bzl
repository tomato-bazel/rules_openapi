"""Go user-facing rules for rules_openapi.

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
"""

load("@rules_go//go:def.bzl", "go_library")

_TOOLCHAIN = "@rules_openapi//openapi:go_client_codegen_toolchain_type"

def _go_client_codegen_impl(ctx):
    out = ctx.actions.declare_file(ctx.label.name + ".go")
    tc = ctx.toolchains[_TOOLCHAIN].codegen_info

    cmd_parts = [
        tc.binary.path,
        "--schema-name={}".format(ctx.file.spec.basename),
        "--rule-name={}".format(ctx.label.name),
        "--package={}".format(_shell_quote(ctx.attr.package)),
    ]
    cmd_parts.extend(["<", ctx.file.spec.path, ">", out.path])

    ctx.actions.run_shell(
        outputs = [out],
        inputs = [ctx.file.spec],
        tools = [tc.binary],
        command = " ".join(cmd_parts),
        mnemonic = "OpenapiGoClient",
        progress_message = "openapi → go client %s" % ctx.label,
    )
    return [DefaultInfo(files = depset([out]))]

_openapi_go_client_codegen = rule(
    implementation = _go_client_codegen_impl,
    attrs = {
        "spec": attr.label(
            allow_single_file = [".yaml", ".yml", ".json"],
            mandatory = True,
            doc = "OpenAPI document (YAML or JSON).",
        ),
        "package": attr.string(
            mandatory = True,
            doc = "Go package name emitted in the generated file.",
        ),
    },
    toolchains = [_TOOLCHAIN],
)

def _shell_quote(s):
    return "'" + s.replace("'", "'\\''") + "'"

def openapi_go_client(
        name,
        spec,
        package = None,
        importpath = None,
        runtime = None,
        runtime_types = None,
        deps = None,
        visibility = None,
        **go_library_kwargs):
    """Generate a go_library of a typed OpenAPI HTTP client.

    The library exports a `Client` + `ClientWithResponses` with one
    method per OpenAPI operation, plus Go types for `components/schemas`.

    Args:
      name: go_library target name. Consumers add this to `deps`.
      spec: label of an OpenAPI `.yaml` / `.yml` / `.json` document.
      package: Go package name for the generated file. Defaults to a
        sanitized form of `name`.
      importpath: go_library importpath. Defaults to `package`. Override
        when consumers import the client by a specific module path.
      runtime: label of `github.com/oapi-codegen/runtime` (the runtime the
        generated client references for parameter binding). Defaults to
        `@com_github_oapi_codegen_runtime//:runtime`. Consumers using their
        own go_deps universe should thread this through so the generated
        code compiles against the same runtime they build with.
      runtime_types: label of `github.com/oapi-codegen/runtime/types` (the
        Date/UUID/File format types the generated code references when the spec
        uses those formats). Defaults to `@com_github_oapi_codegen_runtime//types`.
        Thread from the same universe as `runtime`.
      deps: extra deps forwarded to go_library (for specs whose generated
        code references additional packages).
      visibility: forwarded to go_library.
      **go_library_kwargs: forwarded to go_library.
    """
    pkg = package or _sanitize_package(name)
    gen_name = name + "_go_gen"
    _openapi_go_client_codegen(
        name = gen_name,
        spec = spec,
        package = pkg,
    )
    rt_deps = [
        runtime or Label("@com_github_oapi_codegen_runtime//:runtime"),
        runtime_types or Label("@com_github_oapi_codegen_runtime//types"),
    ]
    go_library(
        name = name,
        srcs = [":" + gen_name],
        importpath = importpath or pkg,
        deps = rt_deps + (deps or []),
        visibility = visibility,
        **go_library_kwargs
    )

def _sanitize_package(name):
    out = ""
    for ch in name.elems():
        if ch.isalnum():
            out += ch.lower()
    if not out:
        out = "client"
    if out[0].isdigit():
        out = "pkg" + out
    return out
