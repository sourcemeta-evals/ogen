package uniqueitemscompile

import (
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// behaviorTest is the committed assertion file for the runtimecheck package.
// It is stored as a non-Go asset and written next to the generated code at
// run time, so the repository never carries a package that cannot compile on
// its own.
//
//go:embed runtime_behavior_test.go.txt
var behaviorTest []byte

// runtimeSchema drives the behavior test in the runtimecheck package. The
// fixed root type name and named Entry definition make every generated
// identifier deterministic, so the committed behavior test compiles against
// any compliant generator output without adaptation.
const runtimeSchema = `{
  "type": "object",
  "required": [ "entries" ],
  "properties": {
    "entries": {
      "type": "array",
      "uniqueItems": true,
      "items": { "$ref": "#/$defs/Entry" }
    }
  },
  "$defs": {
    "Entry": {
      "type": "object",
      "required": [ "id", "name" ],
      "properties": {
        "id": { "type": "string" },
        "name": { "type": "string" },
        "payload": { "type": "string", "format": "byte" },
        "stamp": { "type": "string", "format": "date-time" },
        "extra": {}
      }
    }
  }
}`

// TestUniqueItemsGeneratedValidationBehavior generates the runtime schema
// into the runtimecheck package, next to its committed behavior test, and
// runs that package, so generated uniqueness validation is exercised through
// the public Validate method.
func TestUniqueItemsGeneratedValidationBehavior(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("generation panicked: %+v", recovered)
		}
	}()

	const dir = "runtimecheck"
	require.NoError(t, os.MkdirAll(dir, 0o755))
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})
	require.NoError(t, os.WriteFile(filepath.Join(dir, "runtime_behavior_test.go"), behaviorTest, 0o644))

	generateStandaloneSchema(t, runtimeSchema, dir, "runtimecheck")

	command := exec.Command("go", "test", "-count=1", "./"+dir)
	output, err := command.CombinedOutput()
	require.NoError(t, err, "go test output:\n%s", output)
}
