package validate

import (
	"net"
	"testing"
	"time"

	"github.com/go-faster/jx"
	"github.com/shopspring/decimal"
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
		{Name: "RepeatedMemberVersusSingle", Left: `{"a":1,"a":2}`, Right: `{"a":2}`, Equal: false},
		{Name: "RepeatedMemberLastWins", Left: `{"a":1,"a":2}`, Right: `{"a":0,"a":2}`, Equal: true},
		{Name: "RepeatedMemberDifferentLast", Left: `{"a":1,"a":2}`, Right: `{"a":2,"a":1}`, Equal: false},
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
	t.Run("CyclicValuesSurfaceAnError", func(t *testing.T) {
		type node struct {
			Label string
			Next  *node
		}
		cycle := func(label string) *node {
			first := &node{Label: label}
			first.Next = &node{Label: label, Next: first}
			return first
		}
		// Independently allocated cycles never resolve by pointer identity,
		// so the comparison must stop at the depth budget and report it
		// instead of accepting or rejecting the pair silently
		err := UniqueItemsDeep([]*node{cycle("a"), cycle("a")})
		require.Error(t, err)
		var depthErr *DepthLimitError
		require.ErrorAs(t, err, &depthErr)

		err = UniqueItemsDeep([]*node{cycle("a"), cycle("b")})
		require.NoError(t, err)
	})
	t.Run("CyclesThroughWrapperMethodsSurfaceAnError", func(t *testing.T) {
		cycle := func() nilWrapperValue {
			values := map[string]nilWrapperValue{}
			wrapper := nilWrapperValue{Value: values}
			values["self"] = wrapper
			return wrapper
		}
		// Every pass through the wrapper method re-enters the deep
		// comparison, so a restarted budget would never run out and the
		// stack would be exhausted instead of reporting the depth limit
		err := UniqueItemsDeep([]nilWrapperValue{cycle(), cycle()})
		var depthErr *DepthLimitError
		require.ErrorAs(t, err, &depthErr)

		err = UniqueItemsDeep([]nilWrapperValue{cycle(), {Null: true}})
		require.NoError(t, err)
	})
	t.Run("DepthBeyondBudgetSurfacesAnError", func(t *testing.T) {
		type chain struct {
			Next *chain
		}
		build := func() *chain {
			head := &chain{}
			current := head
			for i := 0; i < equalDeepMaxDepth+1; i++ {
				current.Next = &chain{}
				current = current.Next
			}
			return head
		}
		err := UniqueItemsDeep([]*chain{build(), build()})
		var depthErr *DepthLimitError
		require.ErrorAs(t, err, &depthErr)
	})
}

func TestEqualDeepPanicsOnCycles(t *testing.T) {
	type node struct {
		Next *node
	}
	cyclic := &node{}
	cyclic.Next = cyclic
	other := &node{}
	other.Next = other
	defer func() {
		recovered := recover()
		require.NotNil(t, recovered)
		depthErr, ok := recovered.(*DepthLimitError)
		require.True(t, ok)
		require.Equal(t, equalDeepMaxDepth, depthErr.MaxDepth)
	}()
	EqualDeep(cyclic, other)
}

func TestEqualTime(t *testing.T) {
	parse := func(layout, value string) time.Time {
		parsed, err := time.Parse(layout, value)
		require.NoError(t, err)
		return parsed
	}
	t.Run("SameLiteralParsedTwice", func(t *testing.T) {
		require.True(t, EqualTime(
			parse(time.RFC3339, "2024-01-02T03:04:05+01:23"),
			parse(time.RFC3339, "2024-01-02T03:04:05+01:23"),
		))
	})
	t.Run("FractionalSecondsStayDistinct", func(t *testing.T) {
		require.False(t, EqualTime(
			parse(time.RFC3339, "2024-01-02T03:04:05Z"),
			parse(time.RFC3339Nano, "2024-01-02T03:04:05.000000001Z"),
		))
	})
	t.Run("SameInstantDifferentOffsetStaysDistinct", func(t *testing.T) {
		require.False(t, EqualTime(
			parse(time.RFC3339, "2024-01-02T03:04:05Z"),
			parse(time.RFC3339, "2024-01-02T02:04:05-01:00"),
		))
	})
	t.Run("EquivalentFixedZones", func(t *testing.T) {
		instant := parse(time.RFC3339, "2024-01-02T03:04:05Z")
		require.True(t, EqualTime(
			instant.In(time.FixedZone("A", 3600)),
			instant.In(time.FixedZone("B", 3600)),
		))
	})
}

// namedRaw mirrors a generated alias of a free-form schema, including the
// generated semantic Equal method the deep comparison dispatches to.
type namedRaw jx.Raw

func (s namedRaw) Equal(v namedRaw) bool {
	return EqualRawJSON(jx.Raw(s), jx.Raw(v))
}

// namedBytes mirrors a generated alias of a binary (format byte) schema,
// which has no Equal method and must stay bytewise.
type namedBytes []byte

type equalMethodValue struct {
	ID    string
	Extra jx.Raw
}

func (a equalMethodValue) Equal(b equalMethodValue) bool {
	return a.ID == b.ID && EqualRawJSON(a.Extra, b.Extra)
}

// nilWrapperValue mirrors a generated nullable wrapper: EqualDepth receives
// the current comparison depth and forwards it through EqualDeepDepth, so
// the recursion budget is not restarted at the wrapper boundary.
type nilWrapperValue struct {
	Value map[string]nilWrapperValue
	Null  bool
}

func (s nilWrapperValue) EqualDepth(v nilWrapperValue, depth int) bool {
	if s.Null != v.Null {
		return false
	}
	if s.Null {
		return true
	}
	return EqualDeepDepth(s.Value, v.Value, depth+1)
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
	t.Run("NamedBinarySliceStaysBytewise", func(t *testing.T) {
		// Decoded binary payloads that happen to parse as equivalent JSON
		// numbers are still different bytes
		require.False(t, EqualDeep(
			[]namedBytes{namedBytes(`1`)},
			[]namedBytes{namedBytes(`1.0`)},
		))
		require.True(t, EqualDeep(
			[]namedBytes{namedBytes(`1`)},
			[]namedBytes{namedBytes(`1`)},
		))
	})
	t.Run("HardwareAddrStaysBytewise", func(t *testing.T) {
		require.False(t, EqualDeep(
			net.HardwareAddr([]byte("100000")),
			net.HardwareAddr([]byte("1e0005")),
		))
	})
	t.Run("TimeByInstantAndOffset", func(t *testing.T) {
		first, err := time.Parse(time.RFC3339, "2024-01-02T03:04:05+01:23")
		require.NoError(t, err)
		second, err := time.Parse(time.RFC3339, "2024-01-02T03:04:05+01:23")
		require.NoError(t, err)
		// Separately parsed numeric offsets produce distinct location
		// pointers, so == is false even for identical literals
		require.False(t, first == second)
		require.True(t, EqualDeep(first, second))

		fractional, err := time.Parse(time.RFC3339Nano, "2024-01-02T03:04:05.5+01:23")
		require.NoError(t, err)
		require.False(t, EqualDeep(first, fractional))

		utc, err := time.Parse(time.RFC3339, "2024-01-02T03:04:05Z")
		require.NoError(t, err)
		shifted, err := time.Parse(time.RFC3339, "2024-01-02T02:04:05-01:00")
		require.NoError(t, err)
		require.True(t, utc.Equal(shifted))
		require.False(t, EqualDeep(utc, shifted))
	})
	t.Run("DefinedDecimalTypesCompareNumerically", func(t *testing.T) {
		type money decimal.Decimal
		// Defined types over decimal.Decimal have no promoted Equal method,
		// so without a dedicated case the stored scale would make these
		// numerically equal values distinct
		require.True(t, EqualDeep(
			money(decimal.RequireFromString("1.0")),
			money(decimal.RequireFromString("1.00")),
		))
		require.False(t, EqualDeep(
			money(decimal.RequireFromString("1.0")),
			money(decimal.RequireFromString("1.01")),
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
	t.Run("WrapperDepthMethodDispatch", func(t *testing.T) {
		stale := func(label string) map[string]nilWrapperValue {
			return map[string]nilWrapperValue{label: {Null: true}}
		}
		// Null wrappers compare equal by state, whatever stale payload the
		// inactive Value holds
		require.True(t, EqualDeep(
			nilWrapperValue{Null: true, Value: stale("a")},
			nilWrapperValue{Null: true, Value: stale("b")},
		))
		require.False(t, EqualDeep(
			nilWrapperValue{Null: true},
			nilWrapperValue{Value: stale("a")},
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
