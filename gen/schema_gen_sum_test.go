package gen

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ogen-go/ogen/jsonschema"
)

func TestMergeSchemesConst(t *testing.T) {
	a := require.New(t)

	// Merging a schema with only const (no type, format, or other validators)
	// into a typed schema must preserve the const value.
	s1 := &jsonschema.Schema{
		Type: jsonschema.Integer,
	}
	s2 := &jsonschema.Schema{
		Const:    int64(400),
		ConstSet: true,
	}

	result, err := mergeSchemes(s1, s2)
	a.NoError(err)
	a.True(result.ConstSet, "const must survive allOf merge")
	a.Equal(int64(400), result.Const)
	a.Equal(jsonschema.Integer, result.Type)

	// Merging two schemas with the same const must succeed.
	s3 := &jsonschema.Schema{
		Type:     jsonschema.Integer,
		Const:    int64(400),
		ConstSet: true,
	}
	s4 := &jsonschema.Schema{
		Const:    int64(400),
		ConstSet: true,
	}

	result2, err := mergeSchemes(s3, s4)
	a.NoError(err)
	a.True(result2.ConstSet)
	a.Equal(int64(400), result2.Const)

	// Merging two schemas with different const values must error.
	s5 := &jsonschema.Schema{
		Const:    int64(400),
		ConstSet: true,
	}
	s6 := &jsonschema.Schema{
		Const:    int64(500),
		ConstSet: true,
	}

	_, err = mergeSchemes(s5, s6)
	a.Error(err)
}

func Test_mergeEnums(t *testing.T) {
	tests := []struct {
		a, b    []any
		want    []any
		wantErr bool
	}{
		// Fast path.
		{nil, nil, nil, false},
		{[]any{1, 2, 3}, nil, []any{1, 2, 3}, false},
		// Merge.
		{[]any{1, 2, 3}, []any{3, 4, 5}, []any{3}, false},
		{[]any{3}, []any{3, 4, 5}, []any{3}, false},
		{[]any{"a"}, []any{"b", "a", "c"}, []any{"a"}, false},
		{[]any{
			"a", "b",
			0, 2,
			false, true,
			[]any{1},
			[]any{2},
		}, []any{
			"a", "c",
			0, 3,
			true,
			[]any{1},
			[]any{[]any{1}},
		}, []any{
			"a",
			0,
			true,
			[]any{1},
		}, false},
		// No common values.
		{[]any{1, 2, 3}, []any{4, 5, 6}, nil, true},
	}
	for i, tt := range tests {
		tt := tt
		t.Run(fmt.Sprintf("Test%d", i+1), func(t *testing.T) {
			a := require.New(t)

			// Ensure that merge is commutative.
			got1, err1 := mergeEnums(
				&jsonschema.Schema{Enum: tt.a},
				&jsonschema.Schema{Enum: tt.b},
			)
			got2, err2 := mergeEnums(
				&jsonschema.Schema{Enum: tt.b},
				&jsonschema.Schema{Enum: tt.a},
			)
			if tt.wantErr {
				a.Error(err1)
				a.Error(err2)
				return
			}
			a.NoError(err1)
			a.NoError(err2)
			a.Equal(tt.want, got1)
			a.Equal(tt.want, got2)
		})
	}
}
