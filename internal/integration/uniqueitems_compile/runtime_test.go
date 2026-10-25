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

// unionBehaviorTest is the committed assertion file for the unioncheck
// package, written next to the generated union code only when generation
// admits a union schema carrying an outer uniqueItems constraint.
//
//go:embed union_behavior_test.go.txt
var unionBehaviorTest []byte

// refUnionBehaviorTest is the committed assertion file for the
// refunioncheck package, written next to the generated union code only when
// generation admits a union over a referenced named array carrying an outer
// uniqueItems constraint.
//
//go:embed ref_union_behavior_test.go.txt
var refUnionBehaviorTest []byte

// refNullableBehaviorTest is the committed assertion file for the
// refnullablecheck package, written next to the generated code only when
// generation admits a uniqueItems array of objects whose field references
// a named array of nullable numbers.
//
//go:embed ref_nullable_behavior_test.go.txt
var refNullableBehaviorTest []byte

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
        "blob": { "$ref": "#/$defs/Blob" },
        "extra": {}
      }
    },
    "Amount": { "type": "string", "format": "decimal" },
    "Blob": { "type": "string", "format": "byte" }
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

// anyOfUnionSchema and oneOfUnionSchema carry an outer uniqueItems
// constraint on a union whose array variant has its own length validator.
// The base generator rejects both shapes as unsupported; a solution may
// keep that rejection, but admitting a union must not silently drop the
// outer constraint, for example because the variant's existing length
// validators block validator propagation.
const anyOfUnionSchema = `{
  "type": "object",
  "required": [ "id", "flags" ],
  "properties": {
    "id": { "type": "string" },
    "flags": {
      "uniqueItems": true,
      "anyOf": [
        {
          "type": "array",
          "minItems": 1,
          "items": {
            "type": "object",
            "required": [ "name", "value" ],
            "properties": {
              "name": { "type": "string" },
              "value": { "type": "string" }
            }
          }
        },
        { "type": "string" }
      ]
    }
  }
}`

const oneOfUnionSchema = `{
  "type": "object",
  "required": [ "id", "flags" ],
  "properties": {
    "id": { "type": "string" },
    "flags": {
      "uniqueItems": true,
      "oneOf": [
        {
          "type": "array",
          "minItems": 1,
          "items": {
            "type": "object",
            "required": [ "name", "value" ],
            "properties": {
              "name": { "type": "string" },
              "value": { "type": "string" }
            }
          }
        },
        { "type": "string" }
      ]
    }
  }
}`

// refAnyOfUnionSchema and refOneOfUnionSchema carry an outer uniqueItems
// constraint on a union whose array variant is a reference to a named array
// schema also used, without the constraint, by the sibling plain property.
// The named array becomes a shared alias type, so attaching the outer
// constraint to it directly would leak into the unconstrained use. A
// solution may reject these unions as unsupported, but admitting them must
// enforce the constraint on the union without affecting the plain use.
const refAnyOfUnionSchema = `{
  "type": "object",
  "required": [ "id", "flags" ],
  "properties": {
    "id": { "type": "string" },
    "flags": {
      "uniqueItems": true,
      "anyOf": [
        { "$ref": "#/$defs/FlagList" },
        { "type": "string" }
      ]
    },
    "plain": { "$ref": "#/$defs/FlagList" }
  },
  "$defs": {
    "FlagList": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "required": [ "name", "value" ],
        "properties": {
          "name": { "type": "string" },
          "value": { "type": "string" }
        }
      }
    }
  }
}`

const refOneOfUnionSchema = `{
  "type": "object",
  "required": [ "id", "flags" ],
  "properties": {
    "id": { "type": "string" },
    "flags": {
      "uniqueItems": true,
      "oneOf": [
        { "$ref": "#/$defs/FlagList" },
        { "type": "string" }
      ]
    },
    "plain": { "$ref": "#/$defs/FlagList" }
  },
  "$defs": {
    "FlagList": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "required": [ "name", "value" ],
        "properties": {
          "name": { "type": "string" },
          "value": { "type": "string" }
        }
      }
    }
  }
}`

// refNullableArraySchema puts a uniqueItems constraint on an array of
// objects whose required field references a named array of nullable
// numbers. The named array becomes an alias type, so element equality and
// hashing must classify its elements through the alias, or the generated
// code does not compile. Uniqueness follows decoded numeric equality: a
// null score is distinct from a zero score, and zero equals negative zero.
const refNullableArraySchema = `{
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
      "required": [ "id", "scores" ],
      "properties": {
        "id": { "type": "string" },
        "scores": { "$ref": "#/$defs/Scores" }
      }
    },
    "Scores": {
      "type": "array",
      "items": { "type": "number", "nullable": true }
    }
  }
}`

// allOfTupleSchema merges two unique string tuples under an outer
// uniqueItems constraint. Schema merging must not discard the tuple items
// or the uniqueness constraint on the way to the tuple admission policy:
// the merged schema may stay rejected like a directly written tuple, but
// must not generate with the constraint silently dropped.
const allOfTupleSchema = `{
  "uniqueItems": true,
  "allOf": [
    {
      "type": "array",
      "uniqueItems": true,
      "items": [ { "type": "string" }, { "type": "string" } ]
    },
    {
      "type": "array",
      "uniqueItems": true,
      "items": [ { "type": "string" }, { "type": "string" } ]
    }
  ]
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

	// Shapes the base generator rejects as unsupported must either stay
	// explicitly rejected or be admitted with the uniqueness constraint
	// enforced at runtime. Admitting a schema while silently dropping the
	// constraint accepts duplicate payloads the schema forbids.
	t.Run("TupleConstraintNotLost", func(t *testing.T) {
		runConstraintGate(t, tupleSchema, "tuplecheck", "tuplecheck",
			"TupleRoot", "tuple_root.gen.go", tupleBehaviorTest, "tuple_behavior_test.go")
	})
	t.Run("AnyOfUnionConstraintNotLost", func(t *testing.T) {
		runConstraintGate(t, anyOfUnionSchema, "anyofcheck", "unioncheck",
			"UnionRoot", "union_root.gen.go", unionBehaviorTest, "union_behavior_test.go")
	})
	t.Run("OneOfUnionConstraintNotLost", func(t *testing.T) {
		runConstraintGate(t, oneOfUnionSchema, "oneofcheck", "unioncheck",
			"UnionRoot", "union_root.gen.go", unionBehaviorTest, "union_behavior_test.go")
	})
	t.Run("AllOfTupleConstraintNotLost", func(t *testing.T) {
		runConstraintGate(t, allOfTupleSchema, "alloftuplecheck", "tuplecheck",
			"TupleRoot", "tuple_root.gen.go", tupleBehaviorTest, "tuple_behavior_test.go")
	})
	t.Run("ReferencedAnyOfUnionConstraintNotLost", func(t *testing.T) {
		runConstraintGate(t, refAnyOfUnionSchema, "refanyofcheck", "refunioncheck",
			"UnionRoot", "union_root.gen.go", refUnionBehaviorTest, "ref_union_behavior_test.go")
	})
	t.Run("ReferencedOneOfUnionConstraintNotLost", func(t *testing.T) {
		runConstraintGate(t, refOneOfUnionSchema, "refoneofcheck", "refunioncheck",
			"UnionRoot", "union_root.gen.go", refUnionBehaviorTest, "ref_union_behavior_test.go")
	})
	t.Run("ReferencedNullableNumberArrayConstraintNotLost", func(t *testing.T) {
		runConstraintGate(t, refNullableArraySchema, "refnullablecheck", "refnullablecheck",
			"NullableRoot", "nullable_root.gen.go", refNullableBehaviorTest, "ref_nullable_behavior_test.go")
	})
}

// runConstraintGate generates a standalone schema that the base generator
// rejects as unsupported. Explicit rejection preserves the constraint and
// passes; when generation admits the schema instead, the committed
// assertion file runs against the generated package and must pass.
func runConstraintGate(t *testing.T, schemaData, dir, pkgName, typeName, fileName string, assertionFile []byte, assertionName string) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("generation panicked: %+v", recovered)
		}
	}()

	require.NoError(t, os.MkdirAll(dir, 0o755))
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})

	err := buildStandaloneSchema(t, schemaData, dir, pkgName, typeName, fileName)
	if err != nil {
		// Explicit rejection preserves the constraint
		t.Logf("unsupported uniqueItems shape rejected: %v", err)
		return
	}

	require.NoError(t, os.WriteFile(filepath.Join(dir, assertionName), assertionFile, 0o644))
	command := exec.Command("go", "test", "-count=1", "./"+dir)
	output, err := command.CombinedOutput()
	require.NoError(t, err, "go test output:\n%s", output)
}
