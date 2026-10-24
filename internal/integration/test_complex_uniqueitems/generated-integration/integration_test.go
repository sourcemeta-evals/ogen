package api

import (
	"math"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/ogen-go/ogen/validate"
)

// T053: Integration test combining all field types
func TestValidateUniqueComprehensiveItem_AllFieldTypes(t *testing.T) {
	// Create two different items with all field types populated
	item1 := ComprehensiveItem{
		// Required primitives
		ID:       "item-1",
		Name:     "First Item",
		Priority: 1,
		Active:   true,

		// Optional primitives
		Description: NewOptString("First description"),
		Score:       NewOptFloat64(95.5),

		// Nullable
		ExternalId: NewOptNilString("ext-001"),

		// Enum
		Status: NewOptComprehensiveItemStatus(ComprehensiveItemStatusDraft),

		// Optional enum
		Category: NewOptComprehensiveItemCategory(ComprehensiveItemCategoryFeature),

		// Array of primitives
		Tags: []string{"important", "urgent"},

		// Optional array
		Labels: []string{"frontend", "backend"},

		// Map
		Metadata: NewOptComprehensiveItemMetadata(map[string]string{
			"project": "alpha",
			"team":    "core",
		}),

		// Optional map
		CustomFields: NewOptComprehensiveItemCustomFields(map[string]string{
			"field1": "value1",
		}),

		// Nested object
		Owner: NewOptUser(User{
			Username: "alice",
			Email:    NewOptString("alice@example.com"),
			FullName: NewOptString("Alice Smith"),
			Verified: NewOptBool(true),
		}),

		// Optional nested object
		Assignee: NewOptUser(User{
			Username: "bob",
			Email:    NewOptString("bob@example.com"),
		}),

		// Array of nested objects
		Watchers: []User{
			{Username: "charlie"},
			{Username: "diana"},
		},

		// Complex nested structure
		Configuration: NewOptConfiguration(Configuration{
			Version: NewOptString("1.0"),
			Settings: NewOptConfigurationSettings(map[string]string{
				"theme": "dark",
			}),
			Features: []Feature{
				{
					Name:    NewOptString("experimental"),
					Enabled: NewOptBool(true),
				},
			},
		}),
	}

	item2 := ComprehensiveItem{
		// Different values for all fields
		ID:       "item-2",
		Name:     "Second Item",
		Priority: 2,
		Active:   false,

		Description: NewOptString("Second description"),
		Score:       NewOptFloat64(87.3),

		ExternalId: NewOptNilString("ext-002"),

		Status: NewOptComprehensiveItemStatus(ComprehensiveItemStatusPublished),

		Category: NewOptComprehensiveItemCategory(ComprehensiveItemCategoryBug),

		Tags: []string{"review", "testing"},

		Labels: []string{"api"},

		Metadata: NewOptComprehensiveItemMetadata(map[string]string{
			"project": "beta",
		}),

		CustomFields: NewOptComprehensiveItemCustomFields(map[string]string{
			"field2": "value2",
		}),

		Owner: NewOptUser(User{
			Username: "eve",
			Email:    NewOptString("eve@example.com"),
		}),

		Assignee: NewOptUser(User{
			Username: "frank",
		}),

		Watchers: []User{
			{Username: "grace"},
		},

		Configuration: NewOptConfiguration(Configuration{
			Version: NewOptString("2.0"),
		}),
	}

	items := []ComprehensiveItem{item1, item2}

	// Should pass - items are different
	err := validateUniqueComprehensiveItem(items)
	if err != nil {
		t.Errorf("Expected no error for different items, got: %v", err)
	}
}

// Test duplicate detection with all field types
func TestValidateUniqueComprehensiveItem_DuplicateDetection(t *testing.T) {
	item := ComprehensiveItem{
		ID:       "item-1",
		Name:     "Test Item",
		Priority: 1,
		Active:   true,

		Description: NewOptString("Test description"),
		Score:       NewOptFloat64(100.0),

		Tags: []string{"tag1", "tag2"},

		Metadata: NewOptComprehensiveItemMetadata(map[string]string{
			"key": "value",
		}),

		Owner: NewOptUser(User{
			Username: "user1",
			Email:    NewOptString("user1@example.com"),
		}),

		Watchers: []User{
			{Username: "watcher1"},
		},

		Configuration: NewOptConfiguration(Configuration{
			Version: NewOptString("1.0"),
			Features: []Feature{
				{Name: NewOptString("feat1"), Enabled: NewOptBool(true)},
			},
		}),
	}

	items := []ComprehensiveItem{item, item}

	err := validateUniqueComprehensiveItem(items)
	if err == nil {
		t.Fatal("Expected DuplicateItemsError, got nil")
	}

	dupErr, ok := err.(*validate.DuplicateItemsError)
	if !ok {
		t.Fatalf("Expected *validate.DuplicateItemsError, got %T", err)
	}

	if dupErr.Indices[0] != 0 || dupErr.Indices[1] != 1 {
		t.Errorf("Expected indices [0, 1], got %v", dupErr.Indices)
	}
}

// Test that items differing only in optional fields are detected as different
func TestValidateUniqueComprehensiveItem_OptionalFieldDifferences(t *testing.T) {
	baseItem := ComprehensiveItem{
		ID:       "item-1",
		Name:     "Base Item",
		Priority: 1,
		Active:   true,
		Owner: NewOptUser(User{
			Username: "owner",
		}),
	}

	// Item with optional field set
	itemWithOptional := ComprehensiveItem{
		ID:          "item-1",
		Name:        "Base Item",
		Priority:    1,
		Active:      true,
		Description: NewOptString("Has description"),
		Owner: NewOptUser(User{
			Username: "owner",
		}),
	}

	items := []ComprehensiveItem{baseItem, itemWithOptional}

	err := validateUniqueComprehensiveItem(items)
	if err != nil {
		t.Errorf("Expected no error for items with different optional fields, got: %v", err)
	}
}

// Test that an explicit null nullable field differs from an empty string value
func TestValidateUniqueComprehensiveItem_NullableFieldDifferences(t *testing.T) {
	var nullExternalId OptNilString
	nullExternalId.SetToNull()

	itemWithNull := ComprehensiveItem{
		ID:         "item-1",
		Name:       "Base Item",
		Priority:   1,
		Active:     true,
		ExternalId: nullExternalId,
		Owner: NewOptUser(User{
			Username: "owner",
		}),
	}

	itemWithEmpty := ComprehensiveItem{
		ID:         "item-1",
		Name:       "Base Item",
		Priority:   1,
		Active:     true,
		ExternalId: NewOptNilString(""),
		Owner: NewOptUser(User{
			Username: "owner",
		}),
	}

	items := []ComprehensiveItem{itemWithNull, itemWithEmpty}

	err := validateUniqueComprehensiveItem(items)
	if err != nil {
		t.Errorf("Expected no error for null vs empty string nullable field, got: %v", err)
	}

	var secondNullExternalId OptNilString
	secondNullExternalId.SetToNull()
	duplicateOfNull := itemWithNull
	duplicateOfNull.ExternalId = secondNullExternalId

	duplicates := []ComprehensiveItem{itemWithNull, duplicateOfNull}

	err = validateUniqueComprehensiveItem(duplicates)
	if err == nil {
		t.Error("Expected duplicate error for two items with null nullable fields")
	}
	if _, ok := err.(*validate.DuplicateItemsError); !ok && err != nil {
		t.Errorf("Expected DuplicateItemsError, got: %T", err)
	}
}

// Test that an absent optional array differs from a present empty array
func TestValidateUniqueComprehensiveItem_AbsentVersusEmptyLabels(t *testing.T) {
	itemWithAbsent := ComprehensiveItem{
		ID:       "item-1",
		Name:     "Base Item",
		Priority: 1,
		Active:   true,
		Owner: NewOptUser(User{
			Username: "owner",
		}),
	}

	itemWithEmpty := itemWithAbsent
	itemWithEmpty.Labels = []string{}

	items := []ComprehensiveItem{itemWithAbsent, itemWithEmpty}

	err := validateUniqueComprehensiveItem(items)
	if err != nil {
		t.Errorf("Expected no error for absent vs empty optional array, got: %v", err)
	}

	duplicateOfEmpty := itemWithEmpty
	duplicateOfEmpty.Labels = []string{}

	duplicates := []ComprehensiveItem{itemWithEmpty, duplicateOfEmpty}

	err = validateUniqueComprehensiveItem(duplicates)
	if err == nil {
		t.Error("Expected duplicate error for two items with empty optional arrays")
	}
}

// Test that positive and negative floating point zero hash into the same
// bucket so their equality produces a duplicate error
func TestValidateUniqueComprehensiveItem_NegativeZeroScore(t *testing.T) {
	itemWithPositiveZero := ComprehensiveItem{
		ID:       "item-1",
		Name:     "Base Item",
		Priority: 1,
		Active:   true,
		Score:    NewOptFloat64(0.0),
		Owner: NewOptUser(User{
			Username: "owner",
		}),
	}

	itemWithNegativeZero := itemWithPositiveZero
	itemWithNegativeZero.Score = NewOptFloat64(math.Copysign(0, -1))

	items := []ComprehensiveItem{itemWithPositiveZero, itemWithNegativeZero}

	err := validateUniqueComprehensiveItem(items)
	if err == nil {
		t.Error("Expected duplicate error for items differing only in zero sign")
	}
	if _, ok := err.(*validate.DuplicateItemsError); !ok && err != nil {
		t.Errorf("Expected DuplicateItemsError, got: %T", err)
	}
}

// Decimal-formatted values compare by numeric value after decoding, so
// numerically equal values written with different scales, such as "1.0" and
// "1.00", are duplicates, while different numeric values stay distinct
func TestValidateUniqueComprehensiveItem_StringDecimalNumericEquality(t *testing.T) {
	base := ComprehensiveItem{
		ID:       "item-1",
		Name:     "Base Item",
		Priority: 1,
		Active:   true,
		Owner: NewOptUser(User{
			Username: "owner",
		}),
	}

	oneDigitScale := base
	oneDigitScale.Amount = NewOptStringDecimal(decimal.RequireFromString("1.0"))
	twoDigitScale := base
	twoDigitScale.Amount = NewOptStringDecimal(decimal.RequireFromString("1.00"))

	err := validateUniqueComprehensiveItem([]ComprehensiveItem{oneDigitScale, twoDigitScale})
	if err == nil {
		t.Error("Expected duplicate error for numerically equal decimals")
	}
	if _, ok := err.(*validate.DuplicateItemsError); !ok && err != nil {
		t.Errorf("Expected DuplicateItemsError, got: %T", err)
	}

	larger := base
	larger.Amount = NewOptStringDecimal(decimal.RequireFromString("2.0"))
	err = validateUniqueComprehensiveItem([]ComprehensiveItem{oneDigitScale, larger})
	if err != nil {
		t.Errorf("Expected distinct decimal values to pass, got: %v", err)
	}
}

// Required maps whose values reference a named float alias must hash signed
// zeros identically, or equal items land in different buckets and the
// duplicate escapes detection
func TestValidateUniqueComprehensiveItem_NegativeZeroRatingsMap(t *testing.T) {
	itemWithPositiveZero := ComprehensiveItem{
		ID:       "item-1",
		Name:     "Base Item",
		Priority: 1,
		Active:   true,
		Ratings:  ComprehensiveItemRatings{"overall": 0},
		Owner: NewOptUser(User{
			Username: "owner",
		}),
	}

	itemWithNegativeZero := itemWithPositiveZero
	itemWithNegativeZero.Ratings = ComprehensiveItemRatings{"overall": Rating(math.Copysign(0, -1))}

	err := validateUniqueComprehensiveItem([]ComprehensiveItem{itemWithPositiveZero, itemWithNegativeZero})
	if err == nil {
		t.Error("Expected duplicate error for items differing only in a map value zero sign")
	}
	if _, ok := err.(*validate.DuplicateItemsError); !ok && err != nil {
		t.Errorf("Expected DuplicateItemsError, got: %T", err)
	}

	itemWithOtherValue := itemWithPositiveZero
	itemWithOtherValue.Ratings = ComprehensiveItemRatings{"overall": 1}
	err = validateUniqueComprehensiveItem([]ComprehensiveItem{itemWithPositiveZero, itemWithOtherValue})
	if err != nil {
		t.Errorf("Expected distinct map values to pass, got: %v", err)
	}
}

// A required field referencing a named array of string arrays must compare
// by content, element by element
func TestValidateUniqueComprehensiveItem_NestedRowsAlias(t *testing.T) {
	base := ComprehensiveItem{
		ID:       "item-1",
		Name:     "Base Item",
		Priority: 1,
		Active:   true,
		Rows:     Matrix{{"a", "b"}, {"c"}},
		Owner: NewOptUser(User{
			Username: "owner",
		}),
	}

	duplicate := base
	duplicate.Rows = Matrix{{"a", "b"}, {"c"}}
	err := validateUniqueComprehensiveItem([]ComprehensiveItem{base, duplicate})
	if err == nil {
		t.Error("Expected duplicate error for identical nested rows")
	}
	if _, ok := err.(*validate.DuplicateItemsError); !ok && err != nil {
		t.Errorf("Expected DuplicateItemsError, got: %T", err)
	}

	differentTail := base
	differentTail.Rows = Matrix{{"a", "b"}, {"d"}}
	err = validateUniqueComprehensiveItem([]ComprehensiveItem{base, differentTail})
	if err != nil {
		t.Errorf("Expected rows with different content to pass, got: %v", err)
	}
}

// Referenced number schemas generate named float aliases, whose negative
// zero must hash like positive zero, or equal items land in different
// buckets and the duplicate escapes detection
func TestValidateUniqueComprehensiveItem_NegativeZeroRating(t *testing.T) {
	itemWithPositiveZero := ComprehensiveItem{
		ID:       "item-1",
		Name:     "Base Item",
		Priority: 1,
		Active:   true,
		Rating:   NewOptRating(0),
		Owner: NewOptUser(User{
			Username: "owner",
		}),
	}

	itemWithNegativeZero := itemWithPositiveZero
	itemWithNegativeZero.Rating = NewOptRating(Rating(math.Copysign(0, -1)))

	err := validateUniqueComprehensiveItem([]ComprehensiveItem{itemWithPositiveZero, itemWithNegativeZero})
	if err == nil {
		t.Error("Expected duplicate error for items differing only in the rating zero sign")
	}
	if _, ok := err.(*validate.DuplicateItemsError); !ok && err != nil {
		t.Errorf("Expected DuplicateItemsError, got: %T", err)
	}
}

// Test that items differing only in nested objects are detected as different
func TestValidateUniqueComprehensiveItem_NestedObjectDifferences(t *testing.T) {
	item1 := ComprehensiveItem{
		ID:       "item-1",
		Name:     "Item",
		Priority: 1,
		Active:   true,
		Owner: NewOptUser(User{
			Username: "alice",
			Email:    NewOptString("alice@example.com"),
		}),
	}

	item2 := ComprehensiveItem{
		ID:       "item-1",
		Name:     "Item",
		Priority: 1,
		Active:   true,
		Owner: NewOptUser(User{
			Username: "bob", // Different owner
			Email:    NewOptString("bob@example.com"),
		}),
	}

	items := []ComprehensiveItem{item1, item2}

	err := validateUniqueComprehensiveItem(items)
	if err != nil {
		t.Errorf("Expected no error for items with different nested objects, got: %v", err)
	}
}

// Test that items differing only in arrays are detected as different
func TestValidateUniqueComprehensiveItem_ArrayDifferences(t *testing.T) {
	item1 := ComprehensiveItem{
		ID:       "item-1",
		Name:     "Item",
		Priority: 1,
		Active:   true,
		Tags:     []string{"tag1", "tag2"},
		Owner: NewOptUser(User{
			Username: "owner",
		}),
	}

	item2 := ComprehensiveItem{
		ID:       "item-1",
		Name:     "Item",
		Priority: 1,
		Active:   true,
		Tags:     []string{"tag1", "tag3"}, // Different tags
		Owner: NewOptUser(User{
			Username: "owner",
		}),
	}

	items := []ComprehensiveItem{item1, item2}

	err := validateUniqueComprehensiveItem(items)
	if err != nil {
		t.Errorf("Expected no error for items with different arrays, got: %v", err)
	}
}

// Test that items differing only in maps are detected as different
func TestValidateUniqueComprehensiveItem_MapDifferences(t *testing.T) {
	item1 := ComprehensiveItem{
		ID:       "item-1",
		Name:     "Item",
		Priority: 1,
		Active:   true,
		Metadata: NewOptComprehensiveItemMetadata(map[string]string{
			"key1": "value1",
		}),
		Owner: NewOptUser(User{
			Username: "owner",
		}),
	}

	item2 := ComprehensiveItem{
		ID:       "item-1",
		Name:     "Item",
		Priority: 1,
		Active:   true,
		Metadata: NewOptComprehensiveItemMetadata(map[string]string{
			"key1": "value2", // Different value
		}),
		Owner: NewOptUser(User{
			Username: "owner",
		}),
	}

	items := []ComprehensiveItem{item1, item2}

	err := validateUniqueComprehensiveItem(items)
	if err != nil {
		t.Errorf("Expected no error for items with different maps, got: %v", err)
	}
}

// Test hash and equality consistency
func TestComprehensiveItem_HashEqualConsistency(t *testing.T) {
	item1 := ComprehensiveItem{
		ID:       "item-1",
		Name:     "Test",
		Priority: 1,
		Active:   true,
		Owner: NewOptUser(User{
			Username: "user",
		}),
	}

	item2 := ComprehensiveItem{
		ID:       "item-1",
		Name:     "Test",
		Priority: 1,
		Active:   true,
		Owner: NewOptUser(User{
			Username: "user",
		}),
	}

	item3 := ComprehensiveItem{
		ID:       "item-2",
		Name:     "Test",
		Priority: 1,
		Active:   true,
		Owner: NewOptUser(User{
			Username: "user",
		}),
	}

	// Equal items must have equal hashes
	if item1.Hash(0) != item2.Hash(0) {
		t.Error("Equal items should have equal hashes")
	}

	// Equal items must return true from Equal()
	if !item1.Equal(item2, 0) {
		t.Error("Equal items should return true from Equal()")
	}

	// Different items should return false from Equal()
	if item1.Equal(item3, 0) {
		t.Error("Different items should return false from Equal()")
	}
}

// Test with empty/unset optional fields
func TestValidateUniqueComprehensiveItem_UnsetOptionalFields(t *testing.T) {
	// Minimal items with only required fields
	item1 := ComprehensiveItem{
		ID:       "item-1",
		Name:     "Minimal 1",
		Priority: 1,
		Active:   true,
		Owner: NewOptUser(User{
			Username: "owner1",
		}),
	}

	item2 := ComprehensiveItem{
		ID:       "item-2",
		Name:     "Minimal 2",
		Priority: 2,
		Active:   false,
		Owner: NewOptUser(User{
			Username: "owner2",
		}),
	}

	items := []ComprehensiveItem{item1, item2}

	err := validateUniqueComprehensiveItem(items)
	if err != nil {
		t.Errorf("Expected no error for minimal items, got: %v", err)
	}

	// Test duplicate minimal items
	items = []ComprehensiveItem{item1, item1}
	err = validateUniqueComprehensiveItem(items)
	if err == nil {
		t.Fatal("Expected DuplicateItemsError for duplicate minimal items")
	}
}

// Benchmark with all field types
func BenchmarkValidateUniqueComprehensiveItem_100Items(b *testing.B) {
	items := make([]ComprehensiveItem, 100)
	for i := 0; i < 100; i++ {
		items[i] = ComprehensiveItem{
			ID:       string(rune('A' + (i % 26))),
			Name:     string(rune('a' + (i % 26))),
			Priority: i,
			Active:   i%2 == 0,
			Tags:     []string{string(rune('0' + (i % 10)))},
			Metadata: NewOptComprehensiveItemMetadata(map[string]string{
				"index": string(rune('0' + (i % 10))),
			}),
			Owner: NewOptUser(User{
				Username: string(rune('A' + (i % 26))),
			}),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := validateUniqueComprehensiveItem(items)
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}

// Timestamps parsed separately from the same literal carry distinct location
// pointers, so comparison must use the instant and offset, not ==.
func TestValidateUniqueComprehensiveItem_TimestampDuplicates(t *testing.T) {
	parse := func(layout, value string) time.Time {
		parsed, err := time.Parse(layout, value)
		if err != nil {
			t.Fatalf("parse %q: %v", value, err)
		}
		return parsed
	}
	base := ComprehensiveItem{
		ID:       "item-1",
		Name:     "Timestamped",
		Priority: 1,
		Active:   true,
	}

	first := base
	first.CreatedAt = NewOptDateTime(parse(time.RFC3339, "2024-01-02T03:04:05+01:23"))
	second := base
	second.CreatedAt = NewOptDateTime(parse(time.RFC3339, "2024-01-02T03:04:05+01:23"))
	if err := validateUniqueComprehensiveItem([]ComprehensiveItem{first, second}); err == nil {
		t.Fatal("expected duplicate error for identical timestamps parsed separately")
	}

	fractional := base
	fractional.CreatedAt = NewOptDateTime(parse(time.RFC3339Nano, "2024-01-02T03:04:05.5+01:23"))
	if err := validateUniqueComprehensiveItem([]ComprehensiveItem{first, fractional}); err != nil {
		t.Fatalf("timestamps differing in fractional seconds must stay distinct: %v", err)
	}

	shifted := base
	shifted.CreatedAt = NewOptDateTime(parse(time.RFC3339, "2024-01-02T01:41:05Z"))
	if err := validateUniqueComprehensiveItem([]ComprehensiveItem{first, shifted}); err != nil {
		t.Fatalf("same instant with a different offset must stay distinct: %v", err)
	}
}

// An absent optional byte payload is nil, while a present empty payload is
// an empty non-nil slice, and the two must stay distinct.
func TestValidateUniqueComprehensiveItem_AbsentVersusEmptyPayload(t *testing.T) {
	base := ComprehensiveItem{
		ID:       "item-1",
		Name:     "Payloaded",
		Priority: 1,
		Active:   true,
	}

	absent := base
	present := base
	present.Payload = []byte{}
	if err := validateUniqueComprehensiveItem([]ComprehensiveItem{absent, present}); err != nil {
		t.Fatalf("absent and present empty payloads must stay distinct: %v", err)
	}

	left := base
	left.Payload = []byte{0x01}
	right := base
	right.Payload = []byte{0x01}
	if err := validateUniqueComprehensiveItem([]ComprehensiveItem{left, right}); err == nil {
		t.Fatal("expected duplicate error for identical payloads")
	}
}
