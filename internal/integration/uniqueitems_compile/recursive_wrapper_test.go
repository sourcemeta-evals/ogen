package uniqueitemscompile

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ogen-go/ogen"
	"github.com/ogen-go/ogen/gen"
)

// TestUniqueItemsRecursiveWrapperPointerCompiles locks the pointer-to-wrapper
// shape: an optional nullable self-reference boxes into a pointer to an Opt
// wrapper, which carries EqualDepth instead of the struct comparison
// methods, so generated equality must dispatch through the wrapper and
// hashing must rely on the pointer's presence alone.
func TestUniqueItemsRecursiveWrapperPointerCompiles(t *testing.T) {
	buildDir(t, generateService(t, "recursive-wrapper-pointer.yaml"))
}

// TestUniqueItemsMethodNameConflictRejected asserts that a schema property
// whose Go field name collides with a generated comparison method is
// rejected with an explicit error instead of emitting code that cannot
// compile alongside the field.
func TestUniqueItemsMethodNameConflictRejected(t *testing.T) {
	const document = `
openapi: 3.0.3
info:
  title: Method Name Conflict
  version: 1.0.0
paths:
  /items:
    post:
      operationId: addItems
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: array
              uniqueItems: true
              items:
                type: object
                properties:
                  equal:
                    type: string
                  id:
                    type: string
      responses:
        '200':
          description: Success
`
	spec, err := ogen.Parse([]byte(document))
	require.NoError(t, err)
	_, err = gen.NewGenerator(spec, gen.Options{})
	require.ErrorContains(t, err, "collides with the generated Equal comparison method")
}
