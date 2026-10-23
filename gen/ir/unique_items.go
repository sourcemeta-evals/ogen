package ir

// Dispatch strategies for uniqueItems validation of array types, used by
// the validators template to pick a duplicate detection implementation.
const (
	// UniqueItemsGenerated selects a generated validateUnique function
	// backed by generated Equal and Hash methods on the struct item.
	UniqueItemsGenerated = "generated"
	// UniqueItemsRaw selects semantic raw JSON comparison for free-form
	// jx.Raw items.
	UniqueItemsRaw = "raw"
	// UniqueItemsComparable selects the comparable helper for items the ==
	// operator compares correctly.
	UniqueItemsComparable = "comparable"
	// UniqueItemsDeep selects the semantic deep comparison helper for the
	// remaining item shapes, such as nested arrays, maps and nullable
	// wrappers.
	UniqueItemsDeep = "deep"
)

// unaliased resolves alias chains to the underlying type.
func unaliased(t *Type) *Type {
	for t.Kind == KindAlias && t.AliasTo != nil {
		t = t.AliasTo
	}
	return t
}

// UniqueItemsDispatch returns the duplicate detection strategy for a
// uniqueItems array type.
func (t *Type) UniqueItemsDispatch() string {
	if t.Kind != KindArray || t.Item == nil {
		return ""
	}
	item := t.Item
	switch {
	case item.Is(KindStruct):
		return UniqueItemsGenerated
	case unaliased(item).Is(KindAny):
		// Aliases of free-form schemas are named raw JSON types, which the
		// generic raw helper accepts through its ~[]byte constraint
		return UniqueItemsRaw
	case ComparableWithOperator(item):
		return UniqueItemsComparable
	default:
		return UniqueItemsDeep
	}
}

// ComparableWithOperator reports whether the == operator compiles for the
// type and compares it correctly.
func ComparableWithOperator(t *Type) bool {
	if t == nil {
		return false
	}
	switch t.Kind {
	case KindPrimitive:
		switch t.Primitive {
		case ByteSlice, MAC, File, Custom:
			return false
		case Decimal, URL:
			// Both contain pointers, so == compares identity, not value
			return false
		default:
			return true
		}
	case KindEnum:
		return true
	case KindGeneric:
		return ComparableWithOperator(t.GenericOf)
	case KindAlias:
		return ComparableWithOperator(t.AliasTo)
	default:
		return false
	}
}
