package api

import (
	"encoding/json"
	"testing"

	"github.com/go-faster/jx"
	"github.com/stretchr/testify/require"
)

// TestErrorConstCodeEncoding verifies the const-encoded `code` field on the
// petstore `Error` type. The petstore spec declares `code: {type: integer,
// const: 400}`, so the generated JSON encoder must emit the fixed value 400
// for that field regardless of the struct field's runtime value.
//
// A no-op encoder that reads `s.Code` instead of hardcoding the const value
// would emit the struct's zero value (0) for a default-initialised Error and
// fail this test. The check runs after `make generate examples`, so it
// exercises the encoder that the agent's template actually produced.
func TestErrorConstCodeEncoding(t *testing.T) {
	var err Error // Code defaults to 0; Message defaults to "".
	var enc jx.Encoder
	err.Encode(&enc)

	var got map[string]any
	require.NoError(t, json.Unmarshal(enc.Bytes(), &got))
	require.Equal(t, float64(400), got["code"],
		"const-encoded field must emit the fixed spec value 400, not the struct's zero value")
}
