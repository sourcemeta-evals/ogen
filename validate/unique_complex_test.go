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
		{Name: "StringEscaping", Left: `"\u0041"`, Right: `"A"`, Equal: true},
		{Name: "StringEscapingDistinct", Left: `"\u0042"`, Right: `"A"`, Equal: false},
		{Name: "NumericNotation", Left: `100`, Right: `1e2`, Equal: true},
		{Name: "FractionNotation", Left: `1`, Right: `1.0`, Equal: true},
		{Name: "SignedZero", Left: `-0`, Right: `0`, Equal: true},
		{Name: "LargeIntegersDistinct", Left: `9007199254740992`, Right: `9007199254740993`, Equal: false},
		{Name: "LargeIntegerExponent", Left: `9007199254740992`, Right: `9.007199254740992e15`, Equal: true},
		{Name: "HugeExponentEquivalent", Left: `1e99999999999999999999`, Right: `10e99999999999999999998`, Equal: true},
		{Name: "HugeExponentDistinct", Left: `1e99999999999999999999`, Right: `1.1e99999999999999999999`, Equal: false},
		{Name: "HugeNegativeExponentEquivalent", Left: `1e-99999999999999999999`, Right: `0.1e-99999999999999999998`, Equal: true},
		{Name: "Int64BoundaryExponentEquivalent", Left: `1e9223372036854775808`, Right: `1.0e9223372036854775808`, Equal: true},
		{Name: "Int64WrapDistinct", Left: `10e9223372036854775807`, Right: `1e-9223372036854775808`, Equal: false},
		{Name: "ZeroWithHugeExponent", Left: `0e99999999999999999999`, Right: `0`, Equal: true},
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

type namedRaw jx.Raw

type equalMethodValue struct {
	ID    string
	Extra jx.Raw
}

func (a equalMethodValue) Equal(b equalMethodValue) bool {
	return a.ID == b.ID && EqualRawJSON(a.Extra, b.Extra)
}

func TestEqualDeep(t *testing.T) {
	t.Run("NestedRawArrays", func(t *testing.T) {
		require.True(t, EqualDeep(
			[]jx.Raw{jx.Raw(`{"a":1,"b":2}`)},
			[]jx.Raw{jx.Raw(`{"b":2,"a":1}`)},
		))
		require.False(t, EqualDeep(
			[]jx.Raw{jx.Raw(`{"a":1}`)},
			[]jx.Raw{jx.Raw(`{"a":2}`)},
		))
	})
	t.Run("NestedRawMaps", func(t *testing.T) {
		require.True(t, EqualDeep(
			map[string]jx.Raw{"k": jx.Raw(`"A"`)},
			map[string]jx.Raw{"k": jx.Raw(`"A"`)},
		))
		require.False(t, EqualDeep(
			map[string]jx.Raw{"k": jx.Raw(`1`)},
			map[string]jx.Raw{"k": jx.Raw(`2`)},
		))
	})
	t.Run("NamedRawType", func(t *testing.T) {
		require.True(t, EqualDeep(
			[]namedRaw{namedRaw(`{"a":1,"b":2}`)},
			[]namedRaw{namedRaw(`{"b":2,"a":1}`)},
		))
	})
	t.Run("PlainByteSliceStaysBytewise", func(t *testing.T) {
		require.False(t, EqualDeep(
			[][]byte{[]byte(`{"a":1}`)},
			[][]byte{[]byte(`{ "a":1 }`)},
		))
		require.True(t, EqualDeep(
			[][]byte{[]byte{0x01, 0x02}},
			[][]byte{[]byte{0x01, 0x02}},
		))
	})
	t.Run("EqualMethodDispatch", func(t *testing.T) {
		require.True(t, EqualDeep(
			equalMethodValue{ID: "x", Extra: jx.Raw(`{"a":1,"b":2}`)},
			equalMethodValue{ID: "x", Extra: jx.Raw(`{"b":2,"a":1}`)},
		))
		require.False(t, EqualDeep(
			equalMethodValue{ID: "x", Extra: jx.Raw(`1`)},
			equalMethodValue{ID: "y", Extra: jx.Raw(`1`)},
		))
	})
	t.Run("StructFieldRecursion", func(t *testing.T) {
		type wrapper struct {
			Value equalMethodValue
			Null  bool
		}
		require.True(t, EqualDeep(
			wrapper{Value: equalMethodValue{ID: "x", Extra: jx.Raw(`[1, 2]`)}},
			wrapper{Value: equalMethodValue{ID: "x", Extra: jx.Raw(`[1,2]`)}},
		))
		require.False(t, EqualDeep(
			wrapper{Null: true},
			wrapper{Null: false},
		))
	})
	t.Run("NilVersusEmpty", func(t *testing.T) {
		require.False(t, EqualDeep([]string(nil), []string{}))
		require.False(t, EqualDeep(map[string]string(nil), map[string]string{}))
		require.True(t, EqualDeep([]string(nil), []string(nil)))
	})
	t.Run("Pointers", func(t *testing.T) {
		one, alsoOne, two := 1, 1, 2
		require.True(t, EqualDeep(&one, &alsoOne))
		require.False(t, EqualDeep(&one, &two))
		require.False(t, EqualDeep(&one, nil))
	})
}

func TestUniqueItemsDeepNestedRaw(t *testing.T) {
	err := UniqueItemsDeep([]map[string]jx.Raw{
		{"a": jx.Raw(`1`), "b": jx.Raw(`{"x":1,"y":2}`)},
		{"a": jx.Raw(`2`), "b": jx.Raw(`{"x":1,"y":2}`)},
		{"b": jx.Raw(`{ "y": 2, "x": 1 }`), "a": jx.Raw(`1`)},
	})
	require.Error(t, err)
	var duplicateErr *DuplicateItemsError
	require.ErrorAs(t, err, &duplicateErr)
	require.Equal(t, []int{0, 2}, duplicateErr.Indices)

	require.NoError(t, UniqueItemsDeep([][]jx.Raw{
		{jx.Raw(`9007199254740992`)},
		{jx.Raw(`9007199254740993`)},
	}))
	require.Error(t, UniqueItemsDeep([][]jx.Raw{
		{jx.Raw(`100`)},
		{jx.Raw(`1e2`)},
	}))
}
