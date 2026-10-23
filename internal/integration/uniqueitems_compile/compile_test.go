// Package uniqueitemscompile verifies that code generated for uniqueItems
// arrays compiles through every supported generation entrypoint: full
// service generation, including operations whose only validator is an
// anonymous unique array, recursive schemas, and standalone schema
// generation. Compilation is checked with go build, so the test depends on
// no generated identifiers.
package uniqueitemscompile

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"testing"

	"github.com/go-faster/yaml"
	"github.com/stretchr/testify/require"

	"github.com/ogen-go/ogen"
	"github.com/ogen-go/ogen/gen"
	"github.com/ogen-go/ogen/gen/genfs"
	"github.com/ogen-go/ogen/internal/urlpath"
	"github.com/ogen-go/ogen/jsonpointer"
	"github.com/ogen-go/ogen/jsonschema"
)

const fixtureDir = "../../../_testdata/examples/complex-uniqueitems"

// standaloneSchema is a bare JSON Schema, not an OpenAPI document, so it is
// embedded here instead of living under _testdata/examples, which the root
// TestGenerate consumes as OpenAPI documents.
const standaloneSchema = `{
  "type": "object",
  "required": [ "entries" ],
  "properties": {
    "entries": {
      "type": "array",
      "uniqueItems": true,
      "items": {
        "type": "object",
        "required": [ "id", "name" ],
        "properties": {
          "id": { "type": "string" },
          "name": { "type": "string" }
        }
      }
    }
  }
}`

// outputDir creates a temporary package directory inside the module, so the
// generated code resolves its imports through the repository go.mod.
func outputDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp(".", "generated")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})
	return dir
}

// buildDir compiles the generated package.
func buildDir(t *testing.T, dir string) {
	t.Helper()
	command := exec.Command("go", "build", "./"+filepath.ToSlash(dir))
	output, err := command.CombinedOutput()
	require.NoError(t, err, "go build output:\n%s", output)
}

// generateService generates a full service package from an OpenAPI document.
// Without ignore rules, any unimplemented feature fails generation instead
// of silently skipping the operation.
func generateService(t *testing.T, specName string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixtureDir, specName))
	require.NoError(t, err)
	spec, err := ogen.Parse(data)
	require.NoError(t, err)

	generator, err := gen.NewGenerator(spec, gen.Options{})
	require.NoError(t, err)

	dir := outputDir(t)
	require.NoError(t, generator.WriteSource(genfs.FormattedSource{Root: dir}, "api"))
	return dir
}

// buildStandaloneSchema generates a standalone type for a JSON Schema into
// the given package directory, mirroring the jschemagen entrypoint. Schema
// parsing and generation errors are returned instead of failing the test,
// so callers can accept an explicit rejection of an unsupported feature.
func buildStandaloneSchema(t *testing.T, schemaData, dir, pkgName, typeName, fileName string) error {
	t.Helper()

	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(schemaData), &root))

	parser := jsonschema.NewParser(jsonschema.Settings{
		Resolver: jsonschema.NewRootResolver(&root),
	})
	var rawSchema jsonschema.RawSchema
	require.NoError(t, root.Decode(&rawSchema))

	abs, err := filepath.Abs("standalone-schema.json")
	require.NoError(t, err)
	rootURL, err := urlpath.URLFromFilePath(abs)
	require.NoError(t, err)

	schema, err := parser.Parse(&rawSchema, jsonpointer.NewResolveCtx(rootURL, jsonpointer.DefaultDepthLimit))
	if err != nil {
		return err
	}

	return gen.GenerateSchema(schema, genfs.FormattedSource{Root: dir}, gen.GenerateSchemaOptions{
		TypeName: typeName,
		FileName: fileName,
		PkgName:  pkgName,
	})
}

// generateStandaloneSchema generates a standalone type for a JSON Schema
// into the given package directory and fails the test on any error.
func generateStandaloneSchema(t *testing.T, schemaData, dir, pkgName string) {
	t.Helper()
	require.NoError(t, buildStandaloneSchema(t, schemaData, dir, pkgName, "UniqueEntries", "unique_entries.gen.go"))
}

func TestUniqueItemsGeneratedCodeCompiles(t *testing.T) {
	guard := func(t *testing.T) {
		if recovered := recover(); recovered != nil {
			t.Fatalf("generation panicked: %+v\n%s", recovered, debug.Stack())
		}
	}
	t.Run("AnonymousOperationArray", func(t *testing.T) {
		defer guard(t)
		buildDir(t, generateService(t, "compile-anonymous-array.yaml"))
	})
	t.Run("RecursiveAdditionalProperties", func(t *testing.T) {
		defer guard(t)
		buildDir(t, generateService(t, "compile-recursive-props.yaml"))
	})
	t.Run("StandaloneSchema", func(t *testing.T) {
		defer guard(t)
		dir := outputDir(t)
		generateStandaloneSchema(t, standaloneSchema, dir, "schemaout")
		buildDir(t, dir)
	})
}
