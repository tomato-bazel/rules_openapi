<!-- Generated with Stardoc: http://skydoc.bazel.build -->

Providers exposed by rules_openapi.

Same shape as rules_jsonschema's `JsonschemaCodegenToolchainInfo` —
the plugin contract is identical (stdin/argv/stdout), the only
difference is the schema content shipped on stdin (OpenAPI document
rather than a JSON Schema).

<a id="OpenapiCodegenToolchainInfo"></a>

## OpenapiCodegenToolchainInfo

<pre>
load("@rules_openapi//openapi:providers.bzl", "OpenapiCodegenToolchainInfo")

OpenapiCodegenToolchainInfo(<a href="#OpenapiCodegenToolchainInfo-binary">binary</a>)
</pre>

An OpenAPI → code codegen tool.

**FIELDS**

| Name  | Description |
| :------------- | :------------- |
| <a id="OpenapiCodegenToolchainInfo-binary"></a>binary |  File: the codegen executable. Invoked with `--schema-name=NAME --rule-name=NAME` plus per-plugin flags the calling rule passes through.    |


