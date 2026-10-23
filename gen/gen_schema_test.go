package gen

import (
	"embed"
	"io/fs"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-faster/errors"
	"github.com/go-faster/yaml"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/ogen-go/ogen/gen/genfs"
	"github.com/ogen-go/ogen/jsonpointer"
	"github.com/ogen-go/ogen/jsonschema"
)

//go:embed _testdata/jsonschema
var testdata embed.FS

func TestGenerateSchema(t *testing.T) {
	logger := zaptest.NewLogger(t)

	require.NoError(t, fs.WalkDir(testdata, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return err
		}
		_, file := filepath.Split(path)

		data, err := fs.ReadFile(testdata, path)
		if err != nil {
			return err
		}

		var root yaml.Node
		if err := yaml.Unmarshal(data, &root); err != nil {
			return errors.Wrap(err, "parse yaml")
		}
		p := jsonschema.NewParser(jsonschema.Settings{
			Resolver: jsonschema.NewRootResolver(&root),
		})

		var rawSchema jsonschema.RawSchema
		if err := root.Decode(&rawSchema); err != nil {
			return errors.Wrap(err, "unmarshal")
		}
		ctx := jsonpointer.NewResolveCtx(&url.URL{Path: "/" + path}, jsonpointer.DefaultDepthLimit)
		schema, err := p.Parse(&rawSchema, ctx)
		if err != nil {
			return errors.Wrap(err, "parse")
		}

		t.Run(strings.TrimSuffix(file, ".json"), func(t *testing.T) {
			require.NoError(t, GenerateSchema(
				schema,
				genfs.CheckFS{},
				GenerateSchemaOptions{
					Logger: logger,
				},
			))
		})

		return nil
	}))
}

// A capturing FileSystem for asserting on generated output.
type capturingFS struct {
	files map[string][]byte
}

func (c *capturingFS) WriteFile(baseName string, source []byte) error {
	if c.files == nil {
		c.files = map[string][]byte{}
	}
	c.files[baseName] = source
	return nil
}

func TestGenerateSchemaComplexUniqueItems(t *testing.T) {
	raw := []byte(`{
		"type": "object",
		"properties": {
			"items": {
				"type": "array",
				"uniqueItems": true,
				"items": {
					"type": "object",
					"properties": {
						"id": {"type": "string"},
						"name": {"type": "string"}
					}
				}
			}
		}
	}`)

	var root yaml.Node
	require.NoError(t, yaml.Unmarshal(raw, &root))
	p := jsonschema.NewParser(jsonschema.Settings{
		Resolver: jsonschema.NewRootResolver(&root),
	})
	var rawSchema jsonschema.RawSchema
	require.NoError(t, root.Decode(&rawSchema))
	ctx := jsonpointer.NewResolveCtx(&url.URL{Path: "/schema.json"}, jsonpointer.DefaultDepthLimit)
	schema, err := p.Parse(&rawSchema, ctx)
	require.NoError(t, err)

	captured := &capturingFS{}
	require.NoError(t, GenerateSchema(
		schema,
		captured,
		GenerateSchemaOptions{
			Logger: zaptest.NewLogger(t),
		},
	))

	// The validators in the main output dispatch struct items to a
	// generated validateUnique function, which must be emitted too
	var all strings.Builder
	for _, content := range captured.files {
		all.Write(content)
	}
	output := all.String()
	require.Contains(t, output, "func validateUnique",
		"standalone schema generation must emit the uniqueness validators")
	require.Contains(t, output, ") Equal(",
		"standalone schema generation must emit the Equal methods")
	require.Contains(t, output, ") Hash(",
		"standalone schema generation must emit the Hash methods")
}
