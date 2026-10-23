package api

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/go-faster/jx"

	"github.com/ogen-go/ogen/validate"
)

func requireDuplicateError(t *testing.T, err error, context string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected a duplicate items error, got nil", context)
	}
	// Validate() aggregates field errors without exposing Unwrap, so fall
	// back to the rendered message when errors.As cannot reach the cause
	var duplicateErr *validate.DuplicateItemsError
	if !errors.As(err, &duplicateErr) && !strings.Contains(err.Error(), "duplicate item") {
		t.Fatalf("%s: expected DuplicateItemsError, got %v", context, err)
	}
}

func requireNoError(t *testing.T, err error, context string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: expected no error, got %v", context, err)
	}
}

// Nested collection fields inside struct items must participate in equality.
func TestValidateUniqueGrid_MatrixDifferences(t *testing.T) {
	first := Grid{ID: "g", Matrix: [][]int{{1, 2}, {3}}}
	second := Grid{ID: "g", Matrix: [][]int{{1, 2}, {4}}}
	requireNoError(t, validateUniqueGrid([]Grid{first, second}), "different matrices")

	duplicate := Grid{ID: "g", Matrix: [][]int{{1, 2}, {3}}}
	requireDuplicateError(t, validateUniqueGrid([]Grid{first, duplicate}), "equal matrices")
}

// Raw JSON fields must compare semantically, not byte by byte.
func TestValidateUniqueGrid_RawJSONFieldReorderedKeys(t *testing.T) {
	first := Grid{ID: "g", Extra: jx.Raw(`{"a":1,"b":2}`)}
	reordered := Grid{ID: "g", Extra: jx.Raw(`{"b":2,"a":1}`)}
	requireDuplicateError(t, validateUniqueGrid([]Grid{first, reordered}), "reordered keys")

	different := Grid{ID: "g", Extra: jx.Raw(`{"a":1,"b":3}`)}
	requireNoError(t, validateUniqueGrid([]Grid{first, different}), "different values")
}

// Large integers in raw JSON fields must not collapse through float64.
func TestValidateUniqueGrid_RawJSONLargeIntegers(t *testing.T) {
	first := Grid{ID: "g", Extra: jx.Raw(`9007199254740992`)}
	second := Grid{ID: "g", Extra: jx.Raw(`9007199254740993`)}
	requireNoError(t, validateUniqueGrid([]Grid{first, second}), "adjacent large integers")
}

// Negative zero float map values equal positive zero, so the hash must not
// split such items into different buckets.
func TestValidateUniqueGrid_WeightsSignedZero(t *testing.T) {
	positive := Grid{ID: "g", Weights: NewOptGridWeights(GridWeights{"w": 0})}
	negative := Grid{ID: "g", Weights: NewOptGridWeights(GridWeights{"w": math.Copysign(0, -1)})}
	requireDuplicateError(t, validateUniqueGrid([]Grid{positive, negative}), "signed zero weights")
}

// Map fields with slice values need recursive comparison.
func TestValidateUniqueGrid_LabelsByGroup(t *testing.T) {
	first := Grid{ID: "g", LabelsByGroup: NewOptGridLabelsByGroup(GridLabelsByGroup{
		"core": {"a", "b"},
	})}
	second := Grid{ID: "g", LabelsByGroup: NewOptGridLabelsByGroup(GridLabelsByGroup{
		"core": {"a", "c"},
	})}
	requireNoError(t, validateUniqueGrid([]Grid{first, second}), "different label values")

	duplicate := Grid{ID: "g", LabelsByGroup: NewOptGridLabelsByGroup(GridLabelsByGroup{
		"core": {"a", "b"},
	})}
	requireDuplicateError(t, validateUniqueGrid([]Grid{first, duplicate}), "equal label maps")
}

// Arrays of arrays are validated through the reflect-based helper.
func TestValidateUniqueCollection_Matrices(t *testing.T) {
	collection := Collection{Matrices: [][]int{{1}, {2}, {1, 2}}}
	requireNoError(t, collection.Validate(), "distinct nested arrays")

	collection = Collection{Matrices: [][]int{{1, 2}, {3}, {1, 2}}}
	requireDuplicateError(t, collection.Validate(), "duplicate nested arrays")

	collection = Collection{Matrices: [][]int{{2, 1}, {1, 2}}}
	requireNoError(t, collection.Validate(), "element order is significant")

	collection = Collection{Matrices: [][]int{{}, {}}}
	requireDuplicateError(t, collection.Validate(), "repeated empty arrays")
}

// Map-only object items compare by entries regardless of insertion order.
func TestValidateUniqueCollection_Tagsets(t *testing.T) {
	collection := Collection{Tagsets: []CollectionTagsetsItem{
		{"a": "1", "b": "2"},
		{"a": "1", "b": "3"},
	}}
	requireNoError(t, collection.Validate(), "different map values")

	collection = Collection{Tagsets: []CollectionTagsetsItem{
		{"a": "1", "b": "2"},
		{"b": "2", "a": "1"},
	}}
	requireDuplicateError(t, collection.Validate(), "equal maps built in different order")
}

// Nullable object items distinguish null from present values.
func TestValidateUniqueCollection_Maybes(t *testing.T) {
	null := NilCollectionMaybesItem{Null: true}
	present := NewNilCollectionMaybesItem(CollectionMaybesItem{ID: NewOptString("x")})
	other := NewNilCollectionMaybesItem(CollectionMaybesItem{ID: NewOptString("y")})

	collection := Collection{Maybes: []NilCollectionMaybesItem{null, present, other}}
	requireNoError(t, collection.Validate(), "null and distinct values")

	collection = Collection{Maybes: []NilCollectionMaybesItem{null, null}}
	requireDuplicateError(t, collection.Validate(), "repeated null items")

	duplicate := NewNilCollectionMaybesItem(CollectionMaybesItem{ID: NewOptString("x")})
	collection = Collection{Maybes: []NilCollectionMaybesItem{present, duplicate}}
	requireDuplicateError(t, collection.Validate(), "repeated present items")
}

// Free-form items compare as JSON values, so equivalent representations are
// duplicates and distinct values stay distinct.
func TestValidateUniqueCollection_Anything(t *testing.T) {
	collection := Collection{Anything: []jx.Raw{
		jx.Raw(`{"a":1,"b":2}`),
		jx.Raw(`{"b":2,"a":1}`),
	}}
	requireDuplicateError(t, collection.Validate(), "reordered object keys")

	collection = Collection{Anything: []jx.Raw{
		jx.Raw(`9007199254740992`),
		jx.Raw(`9007199254740993`),
	}}
	requireNoError(t, collection.Validate(), "adjacent large integers")

	collection = Collection{Anything: []jx.Raw{
		jx.Raw(`[null,1]`),
		jx.Raw(`[null,2]`),
	}}
	requireNoError(t, collection.Validate(), "null before differing elements")

	collection = Collection{Anything: []jx.Raw{
		jx.Raw(`[1, 2]`),
		jx.Raw(`[1,2]`),
	}}
	requireDuplicateError(t, collection.Validate(), "whitespace differences")
}

// Items of anonymous operation body arrays get generated equality support.
func TestValidateUniqueAddGridBatchReqItem(t *testing.T) {
	first := AddGridBatchReqItem{Key: NewOptString("k"), Label: NewOptString("a")}
	second := AddGridBatchReqItem{Key: NewOptString("k"), Label: NewOptString("b")}
	requireNoError(t, validateUniqueAddGridBatchReqItem([]AddGridBatchReqItem{first, second}), "distinct items")

	duplicate := AddGridBatchReqItem{Key: NewOptString("k"), Label: NewOptString("a")}
	requireDuplicateError(t, validateUniqueAddGridBatchReqItem([]AddGridBatchReqItem{first, duplicate}), "equal items")
}
