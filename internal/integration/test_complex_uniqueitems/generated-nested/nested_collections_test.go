package api

import (
	"errors"
	"math"
	"net/url"
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

// Ordinary nested records whose names begin with Opt or Nil must compare as
// plain objects through their generated Equal methods, not as wrappers.
func TestValidateUniqueRecord_WrapperLikeNames(t *testing.T) {
	first := Record{
		ID:        "r",
		Options:   Options{Retries: 3, Verbose: NewOptBool(true)},
		NilStatus: NilStatus{Code: 200, Label: NewOptString("ok")},
	}
	differentOptions := Record{
		ID:        "r",
		Options:   Options{Retries: 5, Verbose: NewOptBool(true)},
		NilStatus: NilStatus{Code: 200, Label: NewOptString("ok")},
	}
	requireNoError(t, validateUniqueRecord([]Record{first, differentOptions}), "different Options contents")

	differentStatus := Record{
		ID:        "r",
		Options:   Options{Retries: 3, Verbose: NewOptBool(true)},
		NilStatus: NilStatus{Code: 404, Label: NewOptString("ok")},
	}
	requireNoError(t, validateUniqueRecord([]Record{first, differentStatus}), "different NilStatus contents")

	duplicate := Record{
		ID:        "r",
		Options:   Options{Retries: 3, Verbose: NewOptBool(true)},
		NilStatus: NilStatus{Code: 200, Label: NewOptString("ok")},
	}
	requireDuplicateError(t, validateUniqueRecord([]Record{first, duplicate}), "equal records")
}

// Required named float maps must hash signed zero the same way the equality
// comparison treats it, or the duplicate lands in a different bucket.
func TestValidateUniqueGrid_ScoresSignedZero(t *testing.T) {
	positive := Grid{ID: "g", Scores: GridScores{"w": 0}}
	negative := Grid{ID: "g", Scores: GridScores{"w": math.Copysign(0, -1)}}
	if positive.Hash(0) != negative.Hash(0) {
		t.Fatalf("items that Equal() treats as equal must hash identically")
	}
	requireDuplicateError(t, validateUniqueGrid([]Grid{positive, negative}), "signed zero scores")
}

// Maps whose values are nullable object wrappers have no generated methods on
// the wrapper, so they compare through the reflect-based path.
func TestValidateUniqueGrid_AttachmentsNullableValues(t *testing.T) {
	null := Grid{ID: "g", Attachments: GridAttachments{
		"a": {Null: true},
	}}
	present := Grid{ID: "g", Attachments: GridAttachments{
		"a": NewNilGridAttachmentsItem(GridAttachmentsItem{Name: NewOptString("doc")}),
	}}
	requireNoError(t, validateUniqueGrid([]Grid{null, present}), "null and present values")

	duplicate := Grid{ID: "g", Attachments: GridAttachments{
		"a": NewNilGridAttachmentsItem(GridAttachmentsItem{Name: NewOptString("doc")}),
	}}
	requireDuplicateError(t, validateUniqueGrid([]Grid{present, duplicate}), "equal attachment maps")
}

// url.URL contains a pointer, so independently parsed equal URLs are unequal
// under == and must compare by value instead.
func TestValidateUniqueGrid_LinkByValue(t *testing.T) {
	first, err := url.Parse("https://user:secret@example.com/grids?page=1")
	if err != nil {
		t.Fatal(err)
	}
	second, err := url.Parse("https://user:secret@example.com/grids?page=1")
	if err != nil {
		t.Fatal(err)
	}

	left := Grid{ID: "g", Link: NewOptURI(*first)}
	right := Grid{ID: "g", Link: NewOptURI(*second)}
	if left.Hash(0) != right.Hash(0) {
		t.Fatalf("equal URLs must hash identically")
	}
	requireDuplicateError(t, validateUniqueGrid([]Grid{left, right}), "independently parsed equal URLs")

	other, err := url.Parse("https://user:secret@example.com/grids?page=2")
	if err != nil {
		t.Fatal(err)
	}
	requireNoError(t, validateUniqueGrid([]Grid{left, {ID: "g", Link: NewOptURI(*other)}}), "different URLs")
}

// Free-form values nested inside array items compare as JSON values even
// when the enclosing item shape uses the deep comparison fallback.
func TestValidateUniqueCollection_FreeMatrix(t *testing.T) {
	collection := Collection{FreeMatrix: [][]jx.Raw{
		{jx.Raw(`{"a":1,"b":2}`)},
		{jx.Raw(`{ "b":2, "a":1 }`)},
	}}
	requireDuplicateError(t, collection.Validate(), "reordered nested raw objects")

	collection = Collection{FreeMatrix: [][]jx.Raw{
		{jx.Raw(`100`)},
		{jx.Raw(`1e2`)},
	}}
	requireDuplicateError(t, collection.Validate(), "equivalent nested raw numbers")

	collection = Collection{FreeMatrix: [][]jx.Raw{
		{jx.Raw(`9007199254740992`)},
		{jx.Raw(`9007199254740993`)},
	}}
	requireNoError(t, collection.Validate(), "adjacent large integers stay distinct")

	collection = Collection{FreeMatrix: [][]jx.Raw{
		{jx.Raw(`1`), jx.Raw(`2`)},
		{jx.Raw(`2`), jx.Raw(`1`)},
	}}
	requireNoError(t, collection.Validate(), "element order is significant")
}

// Free-form additionalProperties values inside map-only items compare as
// JSON values.
func TestValidateUniqueCollection_FreeTagsets(t *testing.T) {
	collection := Collection{FreeTagsets: []CollectionFreeTagsetsItem{
		{"k": jx.Raw(`"A"`)},
		{"k": jx.Raw(`"A"`)},
	}}
	requireDuplicateError(t, collection.Validate(), "escaped string values")

	collection = Collection{FreeTagsets: []CollectionFreeTagsetsItem{
		{"k": jx.Raw(`"A"`)},
		{"k": jx.Raw(`"B"`)},
	}}
	requireNoError(t, collection.Validate(), "distinct string values")
}

// Referenced free-form schemas become named raw types and still compare as
// JSON values.
func TestValidateUniqueCollection_Annotations(t *testing.T) {
	collection := Collection{Annotations: []FreeForm{
		FreeForm(`{"note":"x","level":1}`),
		FreeForm(`{"level":1,"note":"x"}`),
	}}
	requireDuplicateError(t, collection.Validate(), "reordered aliased raw objects")

	collection = Collection{Annotations: []FreeForm{
		FreeForm(`{"note":"x"}`),
		FreeForm(`{"note":"y"}`),
	}}
	requireNoError(t, collection.Validate(), "distinct aliased raw objects")
}

// Raw fields inside nullable items must stay semantic through the deep
// comparison of the wrapper.
func TestValidateUniqueCollection_MaybesRawExtra(t *testing.T) {
	first := NewNilCollectionMaybesItem(CollectionMaybesItem{
		ID:    NewOptString("x"),
		Extra: jx.Raw(`{"a":1,"b":2}`),
	})
	reordered := NewNilCollectionMaybesItem(CollectionMaybesItem{
		ID:    NewOptString("x"),
		Extra: jx.Raw(`{"b":2,"a":1}`),
	})
	collection := Collection{Maybes: []NilCollectionMaybesItem{first, reordered}}
	requireDuplicateError(t, collection.Validate(), "reordered raw extras")

	different := NewNilCollectionMaybesItem(CollectionMaybesItem{
		ID:    NewOptString("x"),
		Extra: jx.Raw(`{"a":1,"b":3}`),
	})
	collection = Collection{Maybes: []NilCollectionMaybesItem{first, different}}
	requireNoError(t, collection.Validate(), "different raw extras")
}
