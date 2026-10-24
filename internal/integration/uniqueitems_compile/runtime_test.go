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

// tupleBehaviorTest is the committed assertion file for the tuplecheck
// package, written next to the generated tuple code only when generation
// admits the tuple schema instead of rejecting it.
//
//go:embed tuple_behavior_test.go.txt
var tupleBehaviorTest []byte

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
        "amount": { "$ref": "#/$defs/Amount" },
        "extra": {}
      }
    },
    "Amount": { "type": "string", "format": "decimal" }
  }
}`

// tupleSchema is a two-element string tuple with uniqueItems. The base
// generator rejects this shape as unsupported; a solution may keep that
// rejection, but must not admit the schema while silently dropping the
// uniqueness constraint.
const tupleSchema = `{
  "type": "array",
  "uniqueItems": true,
  "items": [ { "type": "string" }, { "type": "string" } ]
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

	// A tuple array with uniqueItems must either stay explicitly rejected,
	// like the base generator rejects it, or be admitted with the constraint
	// enforced at runtime. Admitting the schema while silently dropping the
	// constraint accepts duplicate payloads the schema forbids.
	t.Run("TupleConstraintNotLost", func(t *testing.T) {
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Fatalf("tuple generation panicked: %+v", recovered)
			}
		}()

		const tupleDir = "tuplecheck"
		require.NoError(t, os.MkdirAll(tupleDir, 0o755))
		t.Cleanup(func() {
			_ = os.RemoveAll(tupleDir)
		})

		err := buildStandaloneSchema(t, tupleSchema, tupleDir, "tuplecheck", "TupleRoot", "tuple_root.gen.go")
		if err != nil {
			// Explicit rejection preserves the constraint
			t.Logf("tuple uniqueItems rejected: %v", err)
			return
		}

		require.NoError(t, os.WriteFile(filepath.Join(tupleDir, "tuple_behavior_test.go"), tupleBehaviorTest, 0o644))
		command := exec.Command("go", "test", "-count=1", "./"+tupleDir)
		output, err := command.CombinedOutput()
		require.NoError(t, err, "go test output:\n%s", output)
	})
}
