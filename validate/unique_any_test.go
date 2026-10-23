package validate

import (
	"testing"

	"github.com/go-faster/jx"
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
	t.Run("NumberBeyondExactComparison", func(t *testing.T) {
		err := UniqueItemsAny([]thing{
			{ID: 1, Extra: jx.Raw(`1e99999999999999999999`)},
			{ID: 1, Extra: jx.Raw(`10e99999999999999999998`)},
		})
		require.ErrorIs(t, err, errExactNumber)
	})
}
