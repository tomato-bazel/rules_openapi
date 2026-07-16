"""Aion user-facing rule for rules_openapi.

`openapi_aion_manifest` is the OpenAPI → aion graph module-objects codegen rule
(the api_manifest reify path):

  1. Resolves the `aion_manifest_codegen_toolchain_type` toolchain.
  2. Runs the toolchain's binary on the OpenAPI spec (stdin/argv/stdout per
     `//openapi/plugin_contract.md`), producing a `<name>.objects.json` — an
     ARRAY of inline aion module objects (an `api` + `api-operation` spine, one
     resource per operation with the request/response schema closure, and the
     `apis.may_call` capability policy).
  3. The generated `.objects.json` is consumed by an aion module's `objects:`
     list via the compiler's partition mode (one cacheable artifact, no per-object
     `.aion`).

Unlike `openapi_rust_client`, fastverk registers NO default toolchain — the
plugin binary (and the aion object model it emits) ships from aion. A consumer
registers it:

    openapi_codegen_toolchain(
        name = "aion_manifest_codegen",
        binary = "@aion-artifact-ingestion//packages/artifact-ingestion-adapters:openapi_to_aion_manifest",
    )
    toolchain(
        name = "aion_manifest_codegen_toolchain",
        toolchain = ":aion_manifest_codegen",
        toolchain_type = "@rules_openapi//openapi:aion_manifest_codegen_toolchain_type",
    )

and `register_toolchains` it in MODULE.bazel.
"""

_TOOLCHAIN = "@rules_openapi//openapi:aion_manifest_codegen_toolchain_type"

def _shell_quote(s):
    return "'" + s.replace("'", "'\\''") + "'"

def _openapi_aion_manifest_impl(ctx):
    out = ctx.actions.declare_file(ctx.label.name + ".objects.json")
    tc = ctx.toolchains[_TOOLCHAIN].codegen_info

    # Standard contract flags + the manifest flags (omitted when empty so the
    # plugin applies its own defaults). Values are simple identifiers / urls.
    cmd_parts = [
        tc.binary.path,
        "--schema-name={}".format(ctx.file.spec.basename),
        "--rule-name={}".format(ctx.label.name),
    ]
    if ctx.attr.namespace:
        cmd_parts.append("--namespace={}".format(ctx.attr.namespace))
    if ctx.attr.api_name:
        cmd_parts.append("--api-name={}".format(ctx.attr.api_name))
    if ctx.attr.base_url:
        cmd_parts.append("--base-url={}".format(ctx.attr.base_url))
    if ctx.attr.transport:
        cmd_parts.append("--transport={}".format(ctx.attr.transport))
    if ctx.attr.spec_format:
        cmd_parts.append("--spec-format={}".format(ctx.attr.spec_format))
    if ctx.attr.auth_secret_ref:
        cmd_parts.append("--auth-secret-ref={}".format(ctx.attr.auth_secret_ref))
    if ctx.attr.idempotent_conflict_ops:
        cmd_parts.append("--idempotent-conflict-ops={}".format(",".join(ctx.attr.idempotent_conflict_ops)))
    if not ctx.attr.emit_types:
        cmd_parts.append("--emit-types=false")
    if not ctx.attr.emit_capability:
        cmd_parts.append("--emit-capability=false")
    for arg in ctx.attr.extra_args:
        cmd_parts.append(_shell_quote(arg))
    cmd_parts.extend(["<", ctx.file.spec.path, ">", out.path])

    ctx.actions.run_shell(
        outputs = [out],
        inputs = [ctx.file.spec],
        tools = [tc.binary],
        command = " ".join(cmd_parts),
        mnemonic = "OpenapiAionManifest",
        progress_message = "openapi → aion manifest %s" % ctx.label,
    )
    return [DefaultInfo(files = depset([out]))]

openapi_aion_manifest = rule(
    implementation = _openapi_aion_manifest_impl,
    attrs = {
        "spec": attr.label(
            allow_single_file = [".yaml", ".yml", ".json"],
            mandatory = True,
            doc = "OpenAPI document (YAML or JSON).",
        ),
        "namespace": attr.string(
            doc = "Target module namespace (for the resource `with.path`). Defaults to the rule name.",
        ),
        "api_name": attr.string(
            doc = "Endpoint-ref prefix (e.g. `workos`). Defaults to a slug of the spec title.",
        ),
        "base_url": attr.string(doc = "The api base url."),
        "transport": attr.string(
            values = ["", "http", "grpc"],
            default = "",
            doc = "Wire transport (default: the plugin's default, http).",
        ),
        "spec_format": attr.string(
            values = ["", "openapi", "proto"],
            default = "",
            doc = "Source spec format (default: openapi).",
        ),
        "auth_secret_ref": attr.string(
            doc = "The auth secret ref (resolved inside the transport, never a binding scope).",
        ),
        "idempotent_conflict_ops": attr.string_list(
            doc = "operationIds whose conflict (\"already exists\") the transport treats as ok.",
        ),
        "emit_types": attr.bool(
            default = True,
            doc = "Emit the api + api-operation core.types inline (false: source from a base module).",
        ),
        "emit_capability": attr.bool(
            default = True,
            doc = "Emit the apis.may_call capability policy.",
        ),
        "extra_args": attr.string_list(
            doc = "Extra `--key=value` flags appended to the plugin invocation.",
        ),
    },
    toolchains = [_TOOLCHAIN],
    doc = "Generate a `<name>.objects.json` of aion module objects from an OpenAPI spec.",
)
