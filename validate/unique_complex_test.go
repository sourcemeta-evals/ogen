package validate

import (
	"testing"

	"github.com/go-faster/jx"
	"github.com/stretchr/testify/require"
)

func TestEqualRawJSON(t *testing.T) {
	for _, tc := range []struct {
		Name  string
		Left  string
		Right string
		Equal bool
	}{
		{Name: "IdenticalObjects", Left: `{"a":1,"b":2}`, Right: `{"a":1,"b":2}`, Equal: true},
		{Name: "ReorderedMembers", Left: `{"a":1,"b":2}`, Right: `{"b":2,"a":1}`, Equal: true},
		{Name: "Whitespace", Left: `[1,2]`, Right: `[ 1, 2 ]`, Equal: true},
		{Name: "StringEscaping", Left: `"A"`, Right: `"A"`, Equal: true},
		{Name: "NumericNotation", Left: `100`, Right: `1e2`, Equal: true},
		{Name: "FractionNotation", Left: `1`, Right: `1.0`, Equal: true},
		{Name: "SignedZero", Left: `-0`, Right: `0`, Equal: true},
		{Name: "LargeIntegersDistinct", Left: `9007199254740992`, Right: `9007199254740993`, Equal: false},
		{Name: "LargeIntegerExponent", Left: `9007199254740992`, Right: `9.007199254740992e15`, Equal: true},
		{Name: "NestedReorderedMembers", Left: `{"a":{"x":1,"y":[2,3]}}`, Right: `{"a":{"y":[2,3],"x":1}}`, Equal: true},
		{Name: "ArrayOrderSignificant", Left: `[1,2]`, Right: `[2,1]`, Equal: false},
		{Name: "NullThenDifferentElement", Left: `[null,1]`, Right: `[null,2]`, Equal: false},
		{Name: "NullThenSameElement", Left: `[null,1]`, Right: `[null,1]`, Equal: true},
		{Name: "NullMemberValue", Left: `{"a":null,"b":1}`, Right: `{"b":1,"a":null}`, Equal: true},
		{Name: "TypeMismatch", Left: `1`, Right: `"1"`, Equal: false},
		{Name: "MissingMember", Left: `{"a":1}`, Right: `{"a":1,"b":2}`, Equal: false},
		{Name: "ExtraMember", Left: `{"a":1,"b":2}`, Right: `{"a":1}`, Equal: false},
		{Name: "ShorterArray", Left: `[1]`, Right: `[1,2]`, Equal: false},
		{Name: "LongerArray", Left: `[1,2]`, Right: `[1]`, Equal: false},
		{Name: "MalformedIdenticalBytes", Left: `{broken`, Right: `{broken`, Equal: true},
		{Name: "MalformedDifferentBytes", Left: `{broken`, Right: `{also broken`, Equal: false},
	} {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			require.Equal(t, tc.Equal, EqualRawJSON(jx.Raw(tc.Left), jx.Raw(tc.Right)))
			require.Equal(t, tc.Equal, EqualRawJSON(jx.Raw(tc.Right), jx.Raw(tc.Left)))
		})
	}
}

func TestUniqueItemsRawJSON(t *testing.T) {
	duplicate := []jx.Raw{
		jx.Raw(`{"a":1,"b":2}`),
		jx.Raw(`{"c":3}`),
		jx.Raw(`{"b":2,"a":1}`),
	}
	err := UniqueItemsRawJSON(duplicate)
	require.Error(t, err)
	var duplicateErr *DuplicateItemsError
	require.ErrorAs(t, err, &duplicateErr)
	require.Equal(t, []int{0, 2}, duplicateErr.Indices)

	require.NoError(t, UniqueItemsRawJSON([]jx.Raw{
		jx.Raw(`{"a":1}`),
		jx.Raw(`{"a":2}`),
		jx.Raw(`[1,2]`),
		jx.Raw(`[2,1]`),
	}))
}

func TestUniqueItemsDeep(t *testing.T) {
	t.Run("NestedArrays", func(t *testing.T) {
		err := UniqueItemsDeep([][]string{{"a"}, {"b"}, {"a"}})
		require.Error(t, err)
		var duplicateErr *DuplicateItemsError
		require.ErrorAs(t, err, &duplicateErr)
		require.Equal(t, []int{0, 2}, duplicateErr.Indices)

		require.NoError(t, UniqueItemsDeep([][]string{{"a"}, {"b"}, {"a", "b"}}))
	})
	t.Run("EmptyArrays", func(t *testing.T) {
		require.Error(t, UniqueItemsDeep([][]string{{}, {}}))
	})
	t.Run("Maps", func(t *testing.T) {
		first := map[string]string{"a": "1", "b": "2"}
		second := map[string]string{"b": "2", "a": "1"}
		require.Error(t, UniqueItemsDeep([]map[string]string{first, second}))
		require.NoError(t, UniqueItemsDeep([]map[string]string{
			{"a": "1"},
			{"a": "2"},
		}))
	})
	t.Run("ShortArrays", func(t *testing.T) {
		require.NoError(t, UniqueItemsDeep([][]string(nil)))
		require.NoError(t, UniqueItemsDeep([][]string{{"a"}}))
	})
}
