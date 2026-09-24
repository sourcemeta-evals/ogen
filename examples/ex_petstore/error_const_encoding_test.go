package api

import (
	"encoding/json"
	"testing"

	"github.com/go-faster/jx"
	"github.com/stretchr/testify/require"
)

// TestErrorConstCodeEncoding verifies the const-encoded fields on the
// petstore `Error` type. The petstore spec declares const values for the
// integer `code`, string `status`, boolean `fatal`, number `ratio`, and
// the optional string `hint`, so the generated JSON encoder must emit the
// fixed spec values for those fields regardless of the struct's runtime
// values.
//
// A no-op encoder that reads the struct fields instead of hardcoding the
// const values would emit the zero values for a default-initialised Error
// and fail this test. The check runs after `make generate examples`, so it
// exercises the encoder that the agent's template actually produced.
func TestErrorConstCodeEncoding(t *testing.T) {
	var err Error // Every field defaults to its zero value.
	var enc jx.Encoder
	err.Encode(&enc)

	var got map[string]any
	require.NoError(t, json.Unmarshal(enc.Bytes(), &got))
	require.Equal(t, float64(400), got["code"],
		"const-encoded integer field must emit the fixed spec value 400, not the struct's zero value")
	require.Equal(t, "error", got["status"],
		"const-encoded string field must emit the fixed spec value, not the struct's zero value")
	require.Equal(t, true, got["fatal"],
		"const-encoded boolean field must emit the fixed spec value, not the struct's zero value")
	require.Equal(t, float64(0.5), got["ratio"],
		"const-encoded number field must emit the fixed spec value, not the struct's zero value")
	require.Equal(t, "retry", got["hint"],
		"optional const-encoded field must emit the fixed spec value even when unset")
}
