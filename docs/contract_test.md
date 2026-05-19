<!-- Generated with Stardoc: http://skydoc.bazel.build -->

OpenAPI plugin conformance test.

`openapi_plugin_contract_test(name, plugin)` runs the rules_openapi
plugin contract scenarios against any plugin executable. Mirrors
rules_jsonschema's `jsonschema_plugin_contract_test` but with
OpenAPI-flavored fixtures (a minimal OpenAPI 3.1 document instead
of a JSON Schema).

<a id="openapi_plugin_contract_test"></a>

## openapi_plugin_contract_test

<pre>
load("@rules_openapi//openapi:contract_test.bzl", "openapi_plugin_contract_test")

openapi_plugin_contract_test(<a href="#openapi_plugin_contract_test-name">name</a>, <a href="#openapi_plugin_contract_test-plugin">plugin</a>)
</pre>

Run the rules_openapi plugin contract scenarios against a plugin binary.

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="openapi_plugin_contract_test-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="openapi_plugin_contract_test-plugin"></a>plugin |  The plugin binary to test.   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |


