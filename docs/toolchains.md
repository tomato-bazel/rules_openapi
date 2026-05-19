<!-- Generated with Stardoc: http://skydoc.bazel.build -->

Toolchain rules for rules_openapi codegen.

`openapi_codegen_toolchain` wraps a single codegen executable as a
Bazel toolchain. Toolchain types are split per (language, use_case)
pair — Rust clients, Go clients, Rust servers, etc. — so a consumer
can swap one plugin without affecting the rest.

Default toolchains are registered in the per-language directories
(`//rust:BUILD.bazel`, …). To swap an implementation, declare your
own `openapi_codegen_toolchain` and `register_toolchains(...)` it
ahead of rules_openapi's default in your `MODULE.bazel`.

<a id="openapi_codegen_toolchain"></a>

## openapi_codegen_toolchain

<pre>
load("@rules_openapi//openapi:toolchains.bzl", "openapi_codegen_toolchain")

openapi_codegen_toolchain(<a href="#openapi_codegen_toolchain-name">name</a>, <a href="#openapi_codegen_toolchain-binary">binary</a>)
</pre>

Declare an OpenAPI → code codegen executable as a Bazel toolchain.

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="openapi_codegen_toolchain-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="openapi_codegen_toolchain-binary"></a>binary |  The codegen executable. Must accept `--schema-name=NAME --rule-name=NAME` plus any per-plugin flags the calling rule passes through.   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |


