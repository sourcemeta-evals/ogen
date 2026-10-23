package api

import (
	"errors"
	"testing"

	"github.com/ogen-go/ogen/validate"
)

// Equal() ignores the inactive payload under a null wrapper, so the hash
// must ignore it too, or equal items land in different buckets and the
// duplicate escapes detection.
func TestValidateUniqueGoldenItem_InactiveNullPayload(t *testing.T) {
	// SetToNull zeroes the payload, so build the stale state directly: it
	// is reachable for any code that flips Null on an existing wrapper
	stale := OptNilString{Set: true, Null: true, Value: "stale payload"}
	first := GoldenItem{ID: "golden", Name: "item", ExternalRef: stale}
	second := GoldenItem{ID: "golden", Name: "item", ExternalRef: OptNilString{Set: true, Null: true}}

	if first.Hash(0) != second.Hash(0) {
		t.Fatalf("items that Equal() treats as equal must hash identically")
	}

	err := validateUniqueGoldenItem([]GoldenItem{first, second})
	if err == nil {
		t.Fatalf("expected a duplicate items error, got nil")
	}
	var duplicateErr *validate.DuplicateItemsError
	if !errors.As(err, &duplicateErr) {
		t.Fatalf("expected DuplicateItemsError, got %v", err)
	}
}

// A null wrapper and a present empty string are different values and must
// not be conflated by either Equal() or the validator.
func TestValidateUniqueGoldenItem_NullVersusEmpty(t *testing.T) {
	null := GoldenItem{ID: "golden", Name: "item", ExternalRef: OptNilString{Set: true, Null: true}}
	empty := GoldenItem{ID: "golden", Name: "item", ExternalRef: NewOptNilString("")}

	if err := validateUniqueGoldenItem([]GoldenItem{null, empty}); err != nil {
		t.Fatalf("null and present-empty values are distinct, got %v", err)
	}
}
