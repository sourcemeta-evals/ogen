package ir

// EqualityMethodSpec describes how to generate an Equal method for a type.
type EqualityMethodSpec struct {
	// TypeName is the Go type name (e.g., "WorkflowStatus")
	TypeName string

	// Fields describes each field's equality comparison logic
	Fields []FieldEqualitySpec

	// NeedsDepthTracking indicates if this type contains nested objects
	// requiring depth limit enforcement
	NeedsDepthTracking bool

	// MaxDepth is the configured maximum nesting depth (default: 10)
	MaxDepth int
}

// FieldEqualitySpec describes how to compare a single field.
type FieldEqualitySpec struct {
	// FieldName is the struct field name (e.g., "ID", "Description")
	FieldName string

	// FieldType categorizes the field for comparison logic selection
	FieldType FieldTypeCategory

	// GoType is the full Go type (e.g., "string", "OptString", "*NestedObject")
	GoType string

	// IsNested indicates if this field is a nested object requiring recursive Equal() call
	IsNested bool

	// IsMap indicates if the field (or wrapped value for Optional/Nullable) is a map type
	IsMap bool

	// IsMapOfStructs indicates if this is a map field where values are structs
	// requiring recursive Equal() calls instead of direct comparison
	IsMapOfStructs bool

	// IsArray indicates if the field (or wrapped value for Optional/Nullable) is an array type
	IsArray bool

	// IsArrayOfStructs indicates if this is an array field where elements are structs
	IsArrayOfStructs bool

	// IsArrayOfNullable indicates if this is an array field where elements are nullable wrappers (NilT)
	IsArrayOfNullable bool

	// IsNilSemanticArray indicates a direct array field where nil carries
	// meaning (absent property or JSON null), so nil and empty must compare
	// as different values
	IsNilSemanticArray bool

	// IsByteSlice indicates if this field is a binary byte slice ([]byte)
	IsByteSlice bool

	// IsRawJSON indicates if this field holds a raw JSON value (jx.Raw)
	// that must be compared semantically, not byte by byte
	IsRawJSON bool

	// IsDeepOnly indicates the field value (or the wrapped value for
	// Optional/Nullable/Pointer fields) has no == operator, no generated
	// Equal method and no dedicated comparison path, so it is compared
	// with reflect.DeepEqual
	IsDeepOnly bool

	// IsArrayOfRaw indicates an array field whose elements are raw JSON
	// values (jx.Raw) compared semantically element by element
	IsArrayOfRaw bool

	// IsArrayOfDeep indicates an array field whose elements have no ==
	// operator and no generated Equal method, so the whole field is
	// compared with reflect.DeepEqual
	IsArrayOfDeep bool

	// IsMapOfRaw indicates a map field whose values are raw JSON values
	// (jx.Raw) compared semantically value by value
	IsMapOfRaw bool

	// IsMapOfDeep indicates a map field whose values have no == operator
	// and no generated Equal method, so values are compared with
	// reflect.DeepEqual
	IsMapOfDeep bool

	// IsOptionalNullable indicates that the FieldTypeOptional wrapper is an
	// OptNilT (both Set and Null flags present), not a plain OptT
	IsOptionalNullable bool
}

// FieldTypeCategory classifies fields for equality comparison.
type FieldTypeCategory int

const (
	FieldTypePrimitive    FieldTypeCategory = iota // string, int, bool, etc.
	FieldTypeOptional                              // OptT, OptNilT
	FieldTypeNullable                              // NilT
	FieldTypePointer                               // *T
	FieldTypeNestedObject                          // struct with generated Equal()
	FieldTypeArray                                 // []T
	FieldTypeMap                                   // map[K]V
)

// HashMethodSpec describes how to generate a Hash method for a type.
type HashMethodSpec struct {
	// TypeName is the Go type name
	TypeName string

	// Fields describes each field's hashing logic
	Fields []FieldHashSpec

	// UsesNestedHash indicates if this type contains nested objects
	// that provide their own Hash() methods
	UsesNestedHash bool
}

// FieldHashSpec describes how to hash a single field.
type FieldHashSpec struct {
	// FieldName is the struct field name
	FieldName string

	// FieldType categorizes the field for hashing logic selection
	FieldType FieldTypeCategory

	// GoType is the full Go type
	GoType string

	// IsNested indicates if this field has a Hash() method
	IsNested bool

	// IsArrayOfStructs indicates array elements provide their own Hash() method
	IsArrayOfStructs bool

	// IsArrayOfNullable indicates array elements are nullable wrappers (NilT)
	IsArrayOfNullable bool

	// IsMapOfStructs indicates map values provide their own Hash() method
	IsMapOfStructs bool

	// IsMap indicates the wrapped value for Optional/Nullable fields is a map
	IsMap bool

	// IsArray indicates the wrapped value for Optional/Nullable fields is an array
	IsArray bool

	// IsRawJSON indicates the field holds raw JSON whose equality ignores
	// its byte representation, so the bytes must not feed the hash
	IsRawJSON bool

	// IsDeepOnly indicates the field is compared with reflect.DeepEqual, so
	// no formatting of it is guaranteed stable across equal values
	IsDeepOnly bool

	// IsArrayOfRaw indicates array elements are raw JSON values
	IsArrayOfRaw bool

	// IsArrayOfDeep indicates array elements are compared with reflect.DeepEqual
	IsArrayOfDeep bool

	// IsMapOfRaw indicates map values are raw JSON values
	IsMapOfRaw bool

	// IsMapOfDeep indicates map values are compared with reflect.DeepEqual
	IsMapOfDeep bool

	// IsOptionalNullable indicates the FieldTypeOptional wrapper is an
	// OptNilT (both Set and Null flags present), not a plain OptT
	IsOptionalNullable bool
}
