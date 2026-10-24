package validate

import (
	"testing"
	"time"

	"github.com/go-faster/jx"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestUniqueItemsAny(t *testing.T) {
	type thing struct {
		ID    int
		Name  string
		Extra jx.Raw
	}

	t.Run("DuplicateStructs", func(t *testing.T) {
		require.Error(t, UniqueItemsAny([]thing{
			{ID: 1, Name: "a"},
			{ID: 2, Name: "b"},
			{ID: 1, Name: "a"},
		}))
	})
	t.Run("DistinctStructs", func(t *testing.T) {
		require.NoError(t, UniqueItemsAny([]thing{
			{ID: 1, Name: "a"},
			{ID: 1, Name: "b"},
		}))
	})
	t.Run("RawJSONReorderedMembers", func(t *testing.T) {
		require.Error(t, UniqueItemsAny([]thing{
			{ID: 1, Extra: jx.Raw(`{"a":1,"b":2}`)},
			{ID: 1, Extra: jx.Raw(`{ "b":2, "a":1 }`)},
		}))
	})
	t.Run("RawJSONEscaping", func(t *testing.T) {
		require.Error(t, UniqueItemsAny([]thing{
			{ID: 1, Extra: jx.Raw(`"A"`)},
			{ID: 1, Extra: jx.Raw(`"A"`)},
		}))
	})
	t.Run("RawJSONNumericNotation", func(t *testing.T) {
		require.Error(t, UniqueItemsAny([]thing{
			{ID: 1, Extra: jx.Raw(`100`)},
			{ID: 1, Extra: jx.Raw(`1e2`)},
		}))
	})
	t.Run("RawJSONLargeIntegersStayDistinct", func(t *testing.T) {
		require.NoError(t, UniqueItemsAny([]thing{
			{ID: 1, Extra: jx.Raw(`9007199254740992`)},
			{ID: 1, Extra: jx.Raw(`9007199254740993`)},
		}))
	})
	t.Run("RawJSONAbsentVersusNull", func(t *testing.T) {
		require.NoError(t, UniqueItemsAny([]thing{
			{ID: 1},
			{ID: 1, Extra: jx.Raw(`null`)},
		}))
	})
	t.Run("AbsentVersusEmptyArray", func(t *testing.T) {
		type holder struct {
			Values []string
		}
		require.NoError(t, UniqueItemsAny([]holder{
			{Values: nil},
			{Values: []string{}},
		}))
	})
	t.Run("NamedRawTypeReorderedMembers", func(t *testing.T) {
		// Referenced free-form schemas generate named jx.Raw aliases, which
		// must compare as JSON values like jx.Raw itself
		type namedRaw jx.Raw
		type holder struct {
			ID    int
			Extra namedRaw
		}
		require.Error(t, UniqueItemsAny([]holder{
			{ID: 1, Extra: namedRaw(`{"a":1,"b":2}`)},
			{ID: 1, Extra: namedRaw(`{ "b":2, "a":1 }`)},
		}))
		require.NoError(t, UniqueItemsAny([]holder{
			{ID: 1, Extra: namedRaw(`{"a":1}`)},
			{ID: 1, Extra: namedRaw(`{"a":2}`)},
		}))
	})
	t.Run("TimestampsCompareByInstantAndOffset", func(t *testing.T) {
		parse := func(layout, value string) time.Time {
			parsed, err := time.Parse(layout, value)
			require.NoError(t, err)
			return parsed
		}
		type stamped struct {
			ID string
			At time.Time
		}
		// Each parse of a numeric offset allocates a new location, so
		// reflect.DeepEqual would miss this duplicate
		require.Error(t, UniqueItemsAny([]stamped{
			{ID: "a", At: parse(time.RFC3339, "2024-01-02T03:04:05+01:23")},
			{ID: "a", At: parse(time.RFC3339, "2024-01-02T03:04:05+01:23")},
		}))
		require.NoError(t, UniqueItemsAny([]stamped{
			{ID: "a", At: parse(time.RFC3339, "2024-01-02T03:04:05+01:23")},
			{ID: "a", At: parse(time.RFC3339Nano, "2024-01-02T03:04:05.5+01:23")},
		}))
		require.NoError(t, UniqueItemsAny([]stamped{
			{ID: "a", At: parse(time.RFC3339, "2024-01-02T03:04:05+01:23")},
			{ID: "a", At: parse(time.RFC3339, "2024-01-02T01:41:05Z")},
		}))
	})
	t.Run("DecimalsCompareNumerically", func(t *testing.T) {
		// Generated decimal fields hold decimal.Decimal or defined types
		// over it; both compare by numeric value, so equal values written
		// with different scales deduplicate
		type priced struct {
			ID     string
			Amount decimal.Decimal
		}
		require.Error(t, UniqueItemsAny([]priced{
			{ID: "a", Amount: decimal.RequireFromString("1.0")},
			{ID: "a", Amount: decimal.RequireFromString("1.00")},
		}))
		require.NoError(t, UniqueItemsAny([]priced{
			{ID: "a", Amount: decimal.RequireFromString("1.0")},
			{ID: "a", Amount: decimal.RequireFromString("1.01")},
		}))

		type money decimal.Decimal
		type tagged struct {
			ID     string
			Amount money
		}
		require.Error(t, UniqueItemsAny([]tagged{
			{ID: "a", Amount: money(decimal.RequireFromString("2.0"))},
			{ID: "a", Amount: money(decimal.RequireFromString("2.00"))},
		}))
	})
	t.Run("RepeatedRawMembersAreOrderIndependent", func(t *testing.T) {
		repeated := thing{ID: 1, Extra: jx.Raw(`{"a":1,"a":1}`)}
		single := thing{ID: 1, Extra: jx.Raw(`{"a":1}`)}
		forward := UniqueItemsAny([]thing{repeated, single})
		backward := UniqueItemsAny([]thing{single, repeated})
		require.Equal(t, forward == nil, backward == nil)
	})
	t.Run("NumberBeyondExactComparison", func(t *testing.T) {
		err := UniqueItemsAny([]thing{
			{ID: 1, Extra: jx.Raw(`1e99999999999999999999`)},
			{ID: 1, Extra: jx.Raw(`10e99999999999999999998`)},
		})
		require.ErrorIs(t, err, errExactNumber)
	})
}
