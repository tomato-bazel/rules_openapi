// Command openapi_to_go_client emits a typed Go HTTP client from an OpenAPI
// document. It is the default codegen toolchain behind rules_openapi's
// openapi_go_client rule.
//
// It wraps oapi-codegen (github.com/oapi-codegen/oapi-codegen) — the heavy
// lifting (models from components/schemas, a Client + ClientWithResponses per
// operation) lives upstream. This binary is a thin adapter from the rules_openapi
// plugin contract (stdin/argv/stdout) onto oapi-codegen's library API, plus one
// spec-preprocessing pass (see normalizeDiscriminatedUnions) that makes
// discriminated oneOf unions with inline branches generatable.
//
// # Contract
//
// Implements the rules_openapi plugin contract (see
// openapi/plugin_contract.md):
//
//   - stdin:  OpenAPI document (JSON or YAML)
//   - argv:   --schema-name=NAME, --rule-name=NAME, --package=NAME
//   - stdout: generated Go source (a single self-contained file)
//   - stderr: diagnostics
//   - exit:   0 success, non-zero failure
//
// Output is deterministic for a given input: the preprocessing pass visits nodes
// in sorted order and oapi-codegen iterates schemas and operations in sorted
// order, embedding no timestamps.
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/codegen"
	"golang.org/x/tools/go/ast/astutil"
	"gopkg.in/yaml.v3"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "openapi_to_go_client: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	args, err := parseArgs(os.Args[1:])
	if err != nil {
		return err
	}

	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("reading OpenAPI doc from stdin: %w", err)
	}

	// Preprocess the raw document (as a generic tree) before handing it to
	// oapi-codegen. yaml.v3 parses both JSON and YAML (JSON is a YAML subset).
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parsing OpenAPI doc (%s) as JSON/YAML: %w", args.schemaName, err)
	}
	root := documentRoot(&doc)
	if root != nil {
		normalizeDiscriminatedUnions(root)
	}
	normalized, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("re-serializing normalized spec: %w", err)
	}

	// kin-openapi builds the typed model oapi-codegen consumes. External $refs
	// are allowed in-memory; we don't fetch remote refs (stdin is the whole input).
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	spec, err := loader.LoadFromData(normalized)
	if err != nil {
		return fmt.Errorf("loading OpenAPI doc (%s): %w", args.schemaName, err)
	}
	// Reject input that parses as YAML/JSON but isn't an OpenAPI document — the
	// `openapi` version field is mandatory in every OpenAPI 3.x doc. Without this,
	// arbitrary maps would yield an empty (but valid) client and a 0 exit,
	// violating the contract's malformed-input expectation.
	if spec == nil || spec.OpenAPI == "" {
		return fmt.Errorf("not an OpenAPI document (%s): missing the \"openapi\" version field", args.schemaName)
	}

	cfg := codegen.Configuration{
		PackageName: args.pkg,
		Generate: codegen.GenerateOptions{
			Models: true,
			Client: true,
		},
		OutputOptions: codegen.OutputOptions{
			// Optional operation filters — when set, only the matching operations
			// (and, after pruning, the schemas they reach) are generated. Lets a
			// consumer carve a small client out of a large API.
			IncludeTags:         args.includeTags,
			IncludeOperationIDs: args.includeOps,
			// oapi-codegen's default post-processing runs goimports
			// (golang.org/x/tools/imports), which shells out to the `go` tool to
			// resolve imports — unavailable in a hermetic build sandbox. Skip it
			// and gofmt the result ourselves with go/format (a pure library). The
			// generated import block is already tight, so no goimports pruning is
			// needed for the output to compile.
			SkipFmt: true,
		},
	}
	code, err := codegen.Generate(spec, cfg)
	if err != nil {
		return fmt.Errorf("oapi-codegen codegen failed: %w", err)
	}

	formatted, err := pruneAndFormat(code)
	if err != nil {
		return err
	}

	if _, err := os.Stdout.Write(formatted); err != nil {
		return fmt.Errorf("writing generated Go to stdout: %w", err)
	}
	return nil
}

// pruneAndFormat replaces the goimports pass oapi-codegen would normally run
// (which shells out to `go` — unavailable in a hermetic sandbox). oapi-codegen's
// templates emit a broad import block covering every generator (all the server
// frameworks etc.) and rely on goimports to drop the ones a client-only
// generation doesn't use. We do the drop with go/ast (a pure library): collect
// the package qualifiers actually referenced, delete every import whose local
// name isn't among them, then gofmt.
func pruneAndFormat(src string) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "generated.go", src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parsing generated client (codegen emitted invalid Go): %w", err)
	}

	used := map[string]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok {
				used[id.Name] = true
			}
		}
		return true
	})

	// Collect the imports to drop first — DeleteNamedImport mutates f.Imports, so
	// deleting mid-range would invalidate the iteration.
	type del struct{ name, path string }
	var drop []del
	for _, imp := range f.Imports {
		if imp.Path == nil {
			continue
		}
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		name := ""
		if imp.Name != nil {
			name = imp.Name.Name
		}
		local := name
		if local == "" {
			local = packageName(path)
		}
		// Keep blank/dot imports (side effects / dot-scope) untouched.
		if local == "_" || local == "." {
			continue
		}
		if !used[local] {
			drop = append(drop, del{name, path})
		}
	}
	for _, d := range drop {
		astutil.DeleteNamedImport(fset, f, d.name, d.path)
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, f); err != nil {
		return nil, fmt.Errorf("gofmt of generated client failed: %w", err)
	}
	return buf.Bytes(), nil
}

var versionSegment = regexp.MustCompile(`^v\d+$`)
var gopkgVersion = regexp.MustCompile(`\.v\d+$`)

// packageName guesses the package identifier for an unrenamed import path,
// handling Go-module major-version suffixes (`.../chi/v5` → `chi`) and gopkg.in
// versioning (`gopkg.in/yaml.v2` → `yaml`). Good enough for oapi-codegen's fixed
// import set; renamed imports never reach here (we use their explicit name).
func packageName(path string) string {
	segs := strings.Split(path, "/")
	last := segs[len(segs)-1]
	if versionSegment.MatchString(last) && len(segs) >= 2 {
		last = segs[len(segs)-2]
	}
	return gopkgVersion.ReplaceAllString(last, "")
}

// ── Discriminated-union normalization ────────────────────────────────────────
//
// oapi-codegen rejects a `oneOf` carrying a `discriminator` when the branches
// are INLINE schemas (no `$ref`) and there is no `mapping`: it can't map an
// anonymous branch to a discriminator value ("not all schemas were mapped").
// Real specs (e.g. WorkOS) do exactly this — an inline oneOf of objects, each
// tagged by a `const`/single-`enum` value on the discriminator property.
//
// This pass lifts each inline branch to a named `components/schemas` entry keyed
// by that discriminator value, replaces the branch with a `$ref`, and fills in
// the `discriminator.mapping`. oapi-codegen then emits a proper discriminated
// union with cleanly named branch types (e.g. `ValidateApiKey`) — strictly more
// faithful than dropping the discriminator (which yields anonymous `…0`/`…1`
// members). When a branch has no derivable discriminator value (so no safe name
// can be built), the pass drops the discriminator for that one union instead —
// the branches still round-trip as an untagged union — and warns on stderr.

func documentRoot(n *yaml.Node) *yaml.Node {
	if n.Kind == yaml.DocumentNode && len(n.Content) == 1 {
		return n.Content[0]
	}
	return n
}

// normalizeDiscriminatedUnions walks the whole document depth-first (post-order,
// so nested unions are normalized before an enclosing branch is lifted) and
// rewrites every discriminated inline oneOf it finds.
func normalizeDiscriminatedUnions(root *yaml.Node) {
	schemas := ensureComponentsSchemas(root)
	used := map[string]bool{}
	for i := 0; i+1 < len(schemas.Content); i += 2 {
		used[schemas.Content[i].Value] = true
	}
	var lifted []nameNode
	visit(root, func(m *yaml.Node) {
		downgradeNullableType(m)
		nullifyOneOf(m)
		disambiguateGoNames(m)
		transformUnion(m, used, &lifted)
	})
	for _, ln := range lifted {
		schemas.Content = append(schemas.Content, scalar(ln.name), ln.node)
	}
}

type nameNode struct {
	name string
	node *yaml.Node
}

// visit calls fn on every mapping node, children first (post-order).
func visit(n *yaml.Node, fn func(*yaml.Node)) {
	switch n.Kind {
	case yaml.DocumentNode:
		for _, c := range n.Content {
			visit(c, fn)
		}
	case yaml.MappingNode:
		for i := 1; i < len(n.Content); i += 2 {
			visit(n.Content[i], fn)
		}
		fn(n)
	case yaml.SequenceNode:
		for _, c := range n.Content {
			visit(c, fn)
		}
	}
}

func transformUnion(m *yaml.Node, used map[string]bool, lifted *[]nameNode) {
	if m.Kind != yaml.MappingNode {
		return
	}
	disc := mapGet(m, "discriminator")
	oneOf := mapGet(m, "oneOf")
	if disc == nil || oneOf == nil || oneOf.Kind != yaml.SequenceNode {
		return
	}
	propName := scalarField(disc, "propertyName")
	if propName == "" {
		return
	}
	// Already fully $ref-based? Leave it — oapi-codegen handles that shape.
	anyInline := false
	for _, br := range oneOf.Content {
		if mapGet(br, "$ref") == nil {
			anyInline = true
		}
	}
	if !anyInline {
		return
	}

	mapping := mapGet(disc, "mapping")
	pairs := map[string]string{}
	newBranches := make([]*yaml.Node, len(oneOf.Content))
	for i, br := range oneOf.Content {
		if ref := mapGet(br, "$ref"); ref != nil {
			newBranches[i] = br
			continue
		}
		val := discriminatorValue(br, propName)
		if val == "" {
			// No derivable tag → we can't build a safe mapping. Drop the
			// discriminator; the untagged oneOf still generates as a union.
			fmt.Fprintf(os.Stderr, "openapi_to_go_client: oneOf branch %d has no derivable %q value; "+
				"emitting an untagged union (dropping the discriminator)\n", i, propName)
			removeKey(m, "discriminator")
			return
		}
		name := uniqueName(val, used)
		*lifted = append(*lifted, nameNode{name: name, node: br})
		ref := "#/components/schemas/" + name
		newBranches[i] = refNode(ref)
		pairs[val] = ref
	}
	oneOf.Content = newBranches

	// Merge/replace the discriminator mapping.
	if mapping == nil {
		mapping = &yaml.Node{Kind: yaml.MappingNode}
		setKey(disc, "mapping", mapping)
	}
	// deterministic mapping order
	mapping.Content = mapping.Content[:0]
	for _, val := range sortedKeys(pairs) {
		mapping.Content = append(mapping.Content, scalar(val), scalar(pairs[val]))
	}
}

// downgradeNullableType rewrites OpenAPI 3.1's nullable-type array
// (`type: [X, null]`) into the 3.0 form oapi-codegen understands (`type: X` +
// `nullable: true`). oapi-codegen (via kin-openapi) can't resolve a `type` that
// is a sequence — it reports "unhandled Schema type: &[string null]". Only the
// single-non-null-plus-null case is downgraded; a genuine multi-type union
// (e.g. [string, integer]) is left untouched (rare, and not safely a scalar).
func downgradeNullableType(m *yaml.Node) {
	if m.Kind != yaml.MappingNode {
		return
	}
	t := mapGet(m, "type")
	if t == nil || t.Kind != yaml.SequenceNode {
		return
	}
	var nonNull []*yaml.Node
	hasNull := false
	for _, e := range t.Content {
		if e.Kind == yaml.ScalarNode && e.Value == "null" {
			hasNull = true
			continue
		}
		nonNull = append(nonNull, e)
	}
	if !hasNull || len(nonNull) != 1 {
		return
	}
	setKey(m, "type", nonNull[0])
	if mapGet(m, "nullable") == nil {
		setKey(m, "nullable", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"})
	}
}

// disambiguateGoNames resolves Go field-name collisions within an object
// schema. Specs that keep a deprecated camelCase property alongside its
// snake_case replacement (e.g. `createdAt` and `created_at`, as WorkOS does)
// make oapi-codegen emit two fields that both PascalCase to `CreatedAt` — a Go
// "field redeclared" error. We keep both properties (no loss of API surface) and
// add an `x-go-name` override to the non-canonical one(s), which oapi-codegen
// honors verbatim. The canonical member (preferring the non-deprecated,
// snake_case spelling) keeps oapi-codegen's natural name.
func disambiguateGoNames(m *yaml.Node) {
	if m.Kind != yaml.MappingNode {
		return
	}
	props := mapGet(m, "properties")
	if props == nil || props.Kind != yaml.MappingNode {
		return
	}
	groups := map[string][]propEnt{}
	var order []string
	for i := 0; i+1 < len(props.Content); i += 2 {
		key := props.Content[i].Value
		norm := strings.ToLower(nonIdent.ReplaceAllString(key, ""))
		if _, ok := groups[norm]; !ok {
			order = append(order, norm)
		}
		groups[norm] = append(groups[norm], propEnt{i, key})
	}
	used := map[string]bool{}
	for _, norm := range order {
		g := groups[norm]
		canonical := pascalName(g[0].key)
		if len(g) < 2 {
			used[canonical] = true
			continue
		}
		ci := chooseCanonical(props, g[0].keyIdx, g)
		for _, e := range g {
			val := props.Content[e.keyIdx+1]
			if e.keyIdx == ci {
				used[canonical] = true
				continue
			}
			if mapGet(val, "x-go-name") != nil {
				continue
			}
			base := canonical
			if scalarField(val, "deprecated") == "true" {
				base = canonical + "Deprecated"
			} else {
				base = pascalName(e.key) + "Alt"
			}
			alias := base
			for i := 2; used[alias] || alias == canonical; i++ {
				alias = fmt.Sprintf("%s%d", base, i)
			}
			used[alias] = true
			setKey(val, "x-go-name", scalar(alias))
		}
	}
}

// chooseCanonical picks which colliding property keeps oapi-codegen's natural
// name: the first non-deprecated member, preferring a snake_case spelling;
// falls back to the first.
type propEnt struct {
	keyIdx int
	key    string
}

func chooseCanonical(props *yaml.Node, first int, g []propEnt) int {
	best := -1
	for _, e := range g {
		val := props.Content[e.keyIdx+1]
		if scalarField(val, "deprecated") == "true" {
			continue
		}
		if best == -1 || strings.Contains(e.key, "_") {
			best = e.keyIdx
		}
	}
	if best == -1 {
		return first
	}
	return best
}

// pascalName renders a JSON property key as oapi-codegen would name the Go field
// (PascalCase across non-alphanumeric + case boundaries — close enough for the
// x-go-name aliases we mint, which are used verbatim regardless).
func pascalName(s string) string {
	parts := nonIdent.Split(s, -1)
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	if b.Len() == 0 {
		return "Field"
	}
	return b.String()
}

// nullifyOneOf handles OpenAPI 3.1's "X or null" unions, expressed as a `oneOf`
// with a bare `{type: null}` branch. oapi-codegen can't resolve the null-typed
// branch ("unhandled Schema type: &[null]"). We drop the null branch and mark
// the schema `nullable: true` — the 3.0 spelling of the same intent. If exactly
// one branch remains, the oneOf collapses to it (a single-member union is just
// that type); zero remaining leaves a plain nullable schema.
func nullifyOneOf(m *yaml.Node) {
	if m.Kind != yaml.MappingNode {
		return
	}
	oneOf := mapGet(m, "oneOf")
	if oneOf == nil || oneOf.Kind != yaml.SequenceNode {
		return
	}
	var kept []*yaml.Node
	removed := false
	for _, br := range oneOf.Content {
		if isNullTypeSchema(br) {
			removed = true
			continue
		}
		kept = append(kept, br)
	}
	if !removed {
		return
	}
	if mapGet(m, "nullable") == nil {
		setKey(m, "nullable", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"})
	}
	switch len(kept) {
	case 0:
		removeKey(m, "oneOf")
	case 1:
		// Collapse the single remaining branch up into this node so we don't
		// leave a degenerate one-member union. A $ref branch becomes this
		// node's $ref; an inline branch's fields are merged in.
		removeKey(m, "oneOf")
		for i := 0; i+1 < len(kept[0].Content); i += 2 {
			setKey(m, kept[0].Content[i].Value, kept[0].Content[i+1])
		}
	default:
		oneOf.Content = kept
	}
}

func isNullTypeSchema(br *yaml.Node) bool {
	if br == nil || br.Kind != yaml.MappingNode {
		return false
	}
	t := mapGet(br, "type")
	if t == nil {
		return false
	}
	if t.Kind == yaml.ScalarNode {
		return t.Value == "null"
	}
	if t.Kind == yaml.SequenceNode && len(t.Content) == 1 {
		return t.Content[0].Value == "null"
	}
	return false
}

// discriminatorValue extracts the const / single-enum value of the discriminator
// property from an inline object branch (properties.<prop>.const | .enum[0]).
func discriminatorValue(branch *yaml.Node, prop string) string {
	props := mapGet(branch, "properties")
	if props == nil {
		return ""
	}
	p := mapGet(props, prop)
	if p == nil {
		return ""
	}
	if c := mapGet(p, "const"); c != nil && c.Kind == yaml.ScalarNode {
		return c.Value
	}
	if e := mapGet(p, "enum"); e != nil && e.Kind == yaml.SequenceNode && len(e.Content) == 1 {
		return e.Content[0].Value
	}
	return ""
}

var nonIdent = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// uniqueName turns a discriminator value into a unique PascalCase schema name.
func uniqueName(val string, used map[string]bool) string {
	parts := nonIdent.Split(val, -1)
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	base := b.String()
	if base == "" || (base[0] >= '0' && base[0] <= '9') {
		base = "Variant" + base
	}
	name := base
	for i := 2; used[name]; i++ {
		name = fmt.Sprintf("%s%d", base, i)
	}
	used[name] = true
	return name
}

// ── small yaml.Node helpers ──────────────────────────────────────────────────

func mapGet(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func scalarField(m *yaml.Node, key string) string {
	v := mapGet(m, key)
	if v == nil || v.Kind != yaml.ScalarNode {
		return ""
	}
	return v.Value
}

func setKey(m *yaml.Node, key string, val *yaml.Node) {
	if existing := mapGet(m, key); existing != nil {
		for i := 0; i+1 < len(m.Content); i += 2 {
			if m.Content[i].Value == key {
				m.Content[i+1] = val
				return
			}
		}
	}
	m.Content = append(m.Content, scalar(key), val)
}

func removeKey(m *yaml.Node, key string) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content = append(m.Content[:i], m.Content[i+2:]...)
			return
		}
	}
}

func scalar(v string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Value: v} }

func refNode(ref string) *yaml.Node {
	return &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{scalar("$ref"), scalar(ref)}}
}

func ensureComponentsSchemas(root *yaml.Node) *yaml.Node {
	comps := mapGet(root, "components")
	if comps == nil {
		comps = &yaml.Node{Kind: yaml.MappingNode}
		setKey(root, "components", comps)
	}
	schemas := mapGet(comps, "schemas")
	if schemas == nil {
		schemas = &yaml.Node{Kind: yaml.MappingNode}
		setKey(comps, "schemas", schemas)
	}
	return schemas
}

func sortedKeys(m map[string]string) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	// simple insertion sort — maps here are tiny (union arity)
	for i := 1; i < len(ks); i++ {
		for j := i; j > 0 && ks[j-1] > ks[j]; j-- {
			ks[j-1], ks[j] = ks[j], ks[j-1]
		}
	}
	return ks
}

// ── argv parsing ─────────────────────────────────────────────────────────────

type parsedArgs struct {
	schemaName  string
	ruleName    string
	pkg         string
	includeTags []string
	includeOps  []string
}

// parseArgs reads the standard --schema-name / --rule-name flags plus the
// go-specific --package. Unknown flags are a hard error (per the contract: a
// silently-ignored flag would let a misconfigured option degrade the output).
func parseArgs(argv []string) (parsedArgs, error) {
	var a parsedArgs
	for _, arg := range argv {
		key, val, ok := strings.Cut(arg, "=")
		if !ok {
			return a, fmt.Errorf("malformed flag %q (expected --key=value)", arg)
		}
		switch key {
		case "--schema-name":
			a.schemaName = val
		case "--rule-name":
			a.ruleName = val
		case "--package":
			a.pkg = val
		case "--include-tags":
			a.includeTags = splitList(val)
		case "--include-operations":
			a.includeOps = splitList(val)
		default:
			return a, fmt.Errorf("unknown flag %q", key)
		}
	}
	if a.pkg == "" {
		a.pkg = sanitizePackage(a.ruleName)
	}
	if a.pkg == "" {
		a.pkg = "client"
	}
	return a, nil
}

// splitList parses a comma-separated flag value into a trimmed, non-empty list.
func splitList(v string) []string {
	var out []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// sanitizePackage lowercases and strips non-identifier characters so a Bazel
// target name (which may contain '-', '.', etc.) yields a legal Go package name.
func sanitizePackage(s string) string {
	s = strings.ToLower(nonIdent.ReplaceAllString(s, ""))
	if s != "" && s[0] >= '0' && s[0] <= '9' {
		s = "pkg" + s
	}
	return s
}
