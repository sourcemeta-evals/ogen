package gen

import (
	"sort"

	"github.com/ogen-go/ogen/gen/ir"
)

const (
	prefixOpt  = "Opt"
	prefixNil  = "Nil"
	fieldValue = "Value"
)

// collectEqualitySpecs identifies types that require Equal() and Hash() methods
// for complex uniqueItems validation.
func (g *Generator) collectEqualitySpecs() {
	// Iterate through all types to find arrays with complex uniqueItems.
	// Operation and webhook payloads are walked as additional roots because
	// anonymous request or response arrays are not stored as named types.
	visited := make(map[string]bool)
	rootVisited := make(map[*ir.Type]bool)
	for _, t := range g.tstorage.types {
		g.collectFromType(t, visited, rootVisited)
	}
	for _, op := range g.operations {
		g.collectFromOperation(op, visited, rootVisited)
	}
	for _, op := range g.webhooks {
		g.collectFromOperation(op, visited, rootVisited)
	}

	// Map iteration order is random, so sort by type name to keep the
	// generated output deterministic across runs
	sort.Slice(g.equalitySpecs, func(i, j int) bool {
		return g.equalitySpecs[i].TypeName < g.equalitySpecs[j].TypeName
	})
}

// collectFromOperation walks every type reachable from an operation's
// parameters, request and responses for uniqueItems arrays.
func (g *Generator) collectFromOperation(op *ir.Operation, visited map[string]bool, rootVisited map[*ir.Type]bool) {
	if op == nil {
		return
	}
	for _, param := range op.Params {
		if param != nil {
			g.collectFromType(param.Type, visited, rootVisited)
		}
	}
	if req := op.Request; req != nil {
		g.collectFromType(req.Type, visited, rootVisited)
		for _, media := range req.Contents {
			g.collectFromType(media.Type, visited, rootVisited)
		}
	}
	if resp := op.Responses; resp != nil {
		g.collectFromType(resp.Type, visited, rootVisited)
		collectResponse := func(r *ir.Response) {
			if r == nil {
				return
			}
			g.collectFromType(r.NoContent, visited, rootVisited)
			for _, media := range r.Contents {
				g.collectFromType(media.Type, visited, rootVisited)
			}
		}
		for _, r := range resp.StatusCode {
			collectResponse(r)
		}
		for _, r := range resp.Pattern {
			collectResponse(r)
		}
		collectResponse(resp.Default)
	}
}

// collectFromType recursively checks a type and everything reachable from it
// for uniqueItems arrays
func (g *Generator) collectFromType(t *ir.Type, visited map[string]bool, rootVisited map[*ir.Type]bool) {
	if t == nil || rootVisited[t] {
		return
	}
	rootVisited[t] = true

	// Check if this is an array with unique items
	if t.Kind == ir.KindArray && t.Schema != nil && t.Schema.UniqueItems {
		if needsEqualityMethod(t.Item) {
			// Create spec for the item type
			spec := g.createEqualityMethodSpec(t.Item)
			if spec != nil {
				g.equalitySpecs = append(g.equalitySpecs, spec)
			}
			// Collect specs for all nested types within this item
			g.collectNestedTypes(t.Item, visited)
		}
	}

	switch t.Kind {
	case ir.KindStruct:
		for _, field := range t.Fields {
			g.collectFromType(field.Type, visited, rootVisited)
		}
	case ir.KindSum:
		for _, variant := range t.SumOf {
			g.collectFromType(variant, visited, rootVisited)
		}
	case ir.KindArray, ir.KindMap:
		g.collectFromType(t.Item, visited, rootVisited)
	case ir.KindGeneric:
		g.collectFromType(t.GenericOf, visited, rootVisited)
	case ir.KindAlias:
		g.collectFromType(t.AliasTo, visited, rootVisited)
	case ir.KindPointer:
		g.collectFromType(t.PointerTo, visited, rootVisited)
	}
}

// collectNestedTypes recursively collects all nested types that need Equal/Hash methods
func (g *Generator) collectNestedTypes(t *ir.Type, visited map[string]bool) {
	if t == nil {
		return
	}

	// Prevent infinite recursion on circular dependencies
	if t.Name != "" && visited[t.Name] {
		return
	}

	switch t.Kind {
	case ir.KindGeneric:
		// Generic types like OptT, NilT - unwrap to the underlying type
		if t.GenericOf != nil {
			g.collectNestedTypes(t.GenericOf, visited)
		}
		return

	case ir.KindStruct:
		// Mark as visited to prevent infinite recursion
		if t.Name != "" {
			visited[t.Name] = true
		}

		// Check if already collected
		alreadyExists := false
		for _, existing := range g.equalitySpecs {
			if existing.TypeName == t.Name {
				alreadyExists = true
				break
			}
		}

		// Regular struct - create spec if not already exists
		if !alreadyExists {
			spec := g.createEqualityMethodSpec(t)
			if spec != nil {
				g.equalitySpecs = append(g.equalitySpecs, spec)
				// Debug: log collection
			}
		}

		// Recurse into all fields to find more nested types
		// Debug: log field traversal
		for _, field := range t.Fields {
			g.collectNestedTypes(field.Type, visited)
		}

	case ir.KindArray:
		// Arrays might contain nested objects
		if t.Item != nil {
			g.collectNestedTypes(t.Item, visited)
		}

	case ir.KindMap:
		// Maps might contain nested objects as values
		if t.Item != nil {
			g.collectNestedTypes(t.Item, visited)
		}

	case ir.KindPointer:
		// Pointers might reference nested objects
		if t.PointerTo != nil {
			g.collectNestedTypes(t.PointerTo, visited)
		}

	case ir.KindAlias:
		if t.AliasTo != nil {
			g.collectNestedTypes(t.AliasTo, visited)
		}
	}
}

// needsEqualityMethod determines if a type requires generated Equal() method.
// It must match the validators template dispatch: only struct items use the
// generated validateUnique function, every other complex item shape is
// compared by the reflect or raw JSON helpers in the validate package.
func needsEqualityMethod(t *ir.Type) bool {
	if t == nil {
		return false
	}
	return t.Kind == ir.KindStruct
}

// createEqualityMethodSpec creates an EqualityMethodSpec for a given type
func (g *Generator) createEqualityMethodSpec(t *ir.Type) *ir.EqualityMethodSpec {
	if t == nil || t.Name == "" {
		return nil
	}

	// Check if we've already created a spec for this type
	for _, existing := range g.equalitySpecs {
		if existing.TypeName == t.Name {
			return nil // Already tracked
		}
	}

	spec := &ir.EqualityMethodSpec{
		TypeName:           t.Name,
		Fields:             []ir.FieldEqualitySpec{},
		NeedsDepthTracking: hasNestedObjects(t),
		MaxDepth:           10, // Default from clarifications
	}

	// Populate fields for struct types
	if t.Kind == ir.KindStruct {
		for _, field := range t.Fields {
			// Unwrap to get the actual type for GoType
			unwrapped := unwrapOptional(field.Type)
			goType := field.Type.Go()
			isMap := false
			isArray := false
			isArrayOfStructs := false
			isByteSlice := false

			if unwrapped != nil && unwrapped != field.Type {
				goType = unwrapped.Go() // Use unwrapped type for better type detection
				isMap = unwrapped.Kind == ir.KindMap
				isArray = unwrapped.Kind == ir.KindArray
			}

			// Check if this is a byte slice (either direct or wrapped)
			if field.Type.Kind == ir.KindPrimitive && field.Type.Primitive == ir.ByteSlice {
				isByteSlice = true
			}
			if unwrapped != nil && unwrapped.Kind == ir.KindPrimitive && unwrapped.Primitive == ir.ByteSlice {
				isByteSlice = true
			}
			if goType == "[]byte" {
				isByteSlice = true
			}

			// Raw JSON values look like byte slices but carry JSON
			// semantics, so they get their own comparison path
			isRawJSON := false
			if goType == "jx.Raw" || field.Type.Kind == ir.KindAny ||
				(unwrapped != nil && unwrapped.Kind == ir.KindAny) {
				isRawJSON = true
				isByteSlice = false
			}

			// Check if this is an array (either direct or wrapped in Optional/Nullable)
			isNilSemanticArray := false
			if field.Type.Kind == ir.KindArray {
				isArray = true
				// Optional and nullable arrays use nil itself to encode the
				// absent or null state, which is distinct from an empty array
				if field.Type.NilSemantic == ir.NilOptional || field.Type.NilSemantic == ir.NilNull {
					isNilSemanticArray = true
				}
			}

			// Check if this is a map of structs (either direct or wrapped in Optional/Nullable)
			isMapOfStructs := false
			mapType := field.Type
			if mapType.Kind != ir.KindMap && unwrapped != nil && unwrapped.Kind == ir.KindMap {
				mapType = unwrapped
			}
			isMapOfFloats := false
			if mapType.Kind == ir.KindMap && mapType.Item != nil {
				// Only direct struct values have generated Equal and Hash
				// methods. Wrapper values, such as nullable objects, have
				// neither and fall through to the reflect-based path
				if mapType.Item.Kind == ir.KindStruct {
					isMapOfStructs = true
				}
				// Resolved from the IR because named map types render their
				// type name, not map[...] syntax, in the Go representation
				if mapType.Item.Kind == ir.KindPrimitive &&
					(mapType.Item.Primitive == ir.Float32 || mapType.Item.Primitive == ir.Float64) {
					isMapOfFloats = true
				}
			}

			// Check if this is an array of structs or nullable wrappers (either direct or wrapped)
			isArrayOfNullable := false
			arrayType := field.Type
			classifyArrayItem := func(item *ir.Type) {
				if item == nil {
					return
				}
				if item.Kind == ir.KindGeneric && item.GenericVariant.OnlyNullable() {
					isArrayOfNullable = true
					// Also check if the VALUE inside the nullable wrapper is a struct
					if inner := unwrapOptional(item); inner != nil && inner.Kind == ir.KindStruct {
						isArrayOfStructs = true
					}
					return
				}
				if item.Kind == ir.KindStruct {
					isArrayOfStructs = true
				}
			}
			if arrayType.Kind == ir.KindArray && arrayType.Item != nil {
				classifyArrayItem(arrayType.Item)
			} else if unwrapped != nil && unwrapped.Kind == ir.KindArray && unwrapped.Item != nil {
				// Wrapped array (OptT[[]Struct] or NilT[[]Struct])
				classifyArrayItem(unwrapped.Item)
			}

			// Classify the remaining array element and map value shapes:
			// raw JSON values get semantic comparison, element types the ==
			// operator cannot compare fall back to reflect.DeepEqual
			isArrayOfRaw := false
			isArrayOfDeep := false
			elemArray := arrayType
			if elemArray.Kind != ir.KindArray && unwrapped != nil && unwrapped.Kind == ir.KindArray {
				elemArray = unwrapped
			}
			if elemArray.Kind == ir.KindArray && elemArray.Item != nil && !isArrayOfStructs {
				elem := elemArray.Item
				switch {
				case elem.Kind == ir.KindAny:
					isArrayOfRaw = true
					isArrayOfNullable = false
				case isArrayOfNullable:
					// Nullable wrapper elements whose value is neither a
					// struct nor comparable cannot use the .Value != .Value
					// path
					if inner := unwrapOptional(elem); inner != nil && !ir.ComparableWithOperator(inner) {
						isArrayOfDeep = true
						isArrayOfNullable = false
					}
				case !ir.ComparableWithOperator(elem):
					isArrayOfDeep = true
				}
			}

			isMapOfRaw := false
			isMapOfDeep := false
			if mapType.Kind == ir.KindMap && mapType.Item != nil && !isMapOfStructs {
				switch {
				case mapType.Item.Kind == ir.KindAny:
					isMapOfRaw = true
				case !ir.ComparableWithOperator(mapType.Item):
					isMapOfDeep = true
				}
			}

			// Formatted scalars whose Go representation contains pointers
			// need value-based comparison instead of == or reflection
			isURL := false
			isDecimal := false
			scalarType := field.Type
			if unwrapped != nil && unwrapped != field.Type {
				scalarType = unwrapped
			}
			if scalarType != nil && scalarType.Kind == ir.KindPrimitive {
				switch scalarType.Primitive {
				case ir.URL:
					isURL = true
				case ir.Decimal:
					isDecimal = true
				}
			}

			// Field values with no == operator, no generated Equal method
			// and no dedicated path above are compared with reflect.DeepEqual
			isDeepOnly := false
			switch categorizeFieldType(field.Type) {
			case ir.FieldTypePrimitive:
				if !isRawJSON && !isByteSlice && !isURL && !isDecimal &&
					!ir.ComparableWithOperator(field.Type) {
					isDeepOnly = true
				}
			case ir.FieldTypeOptional, ir.FieldTypeNullable:
				if unwrapped != nil && !isNestedObject(field.Type) && !isMap && !isArray &&
					!isRawJSON && !isByteSlice && !isURL && !isDecimal &&
					!ir.ComparableWithOperator(unwrapped) {
					isDeepOnly = true
				}
			case ir.FieldTypePointer:
				if field.Type.PointerTo != nil && !isNestedObject(field.Type) &&
					!ir.ComparableWithOperator(field.Type.PointerTo) {
					isDeepOnly = true
				}
			}

			fieldSpec := ir.FieldEqualitySpec{
				FieldName:          field.Name,
				FieldType:          categorizeFieldType(field.Type),
				GoType:             goType,
				IsNested:           isNestedObject(field.Type),
				IsMap:              isMap,
				IsMapOfStructs:     isMapOfStructs,
				IsMapOfFloats:      isMapOfFloats,
				IsMapOfRaw:         isMapOfRaw,
				IsMapOfDeep:        isMapOfDeep,
				IsArray:            isArray,
				IsArrayOfStructs:   isArrayOfStructs,
				IsArrayOfNullable:  isArrayOfNullable,
				IsArrayOfRaw:       isArrayOfRaw,
				IsArrayOfDeep:      isArrayOfDeep,
				IsNilSemanticArray: isNilSemanticArray,
				IsByteSlice:        isByteSlice,
				IsRawJSON:          isRawJSON,
				IsURL:              isURL,
				IsDecimal:          isDecimal,
				IsDeepOnly:         isDeepOnly,
				IsOptionalNullable: isOptionalNullable(field.Type),
			}
			spec.Fields = append(spec.Fields, fieldSpec)
		}
	}

	return spec
}

// hasNestedObjects checks if a type contains nested objects requiring depth tracking
// For simplicity and consistency, all struct types that will have Equal() methods
// should have depth tracking. This ensures uniform Equal() signatures.
func hasNestedObjects(t *ir.Type) bool {
	if t == nil {
		return false
	}

	// All struct types get depth tracking for consistent Equal() signatures
	// This simplifies code generation and calling conventions
	return t.Kind == ir.KindStruct
}

// unwrapOptional unwraps Generic optional/nullable types to get the underlying type.
// Wrappers are always KindGeneric: user-defined structs whose names merely start
// with Opt or Nil are ordinary objects and must not be unwrapped.
func unwrapOptional(t *ir.Type) *ir.Type {
	if t == nil {
		return nil
	}

	// Unwrap generic types (OptT, NilT)
	if t.Kind == ir.KindGeneric && t.GenericOf != nil {
		return t.GenericOf
	}

	return t
}

// isNestedObject checks if a type is a nested object (struct)
func isNestedObject(t *ir.Type) bool {
	if t == nil {
		return false
	}

	switch t.Kind {
	case ir.KindGeneric:
		// Generic types like OptT, NilT - check if they wrap a nested object
		if t.GenericOf != nil {
			return isNestedObject(t.GenericOf)
		}
		return false
	case ir.KindStruct:
		// Every plain struct is a nested object. Wrappers are KindGeneric,
		// so names starting with Opt or Nil carry no meaning here
		return true
	case ir.KindPointer:
		// Pointer to a nested object - compare via Equal() on the pointee
		if t.PointerTo != nil {
			return isNestedObject(t.PointerTo)
		}
		return false
	case ir.KindAlias:
		if t.AliasTo != nil {
			return isNestedObject(t.AliasTo)
		}
		return false
	default:
		return false
	}
}

// isOptionalNullable returns true when t is an OptNilT wrapper, which carries
// both Set and Null flags. Plain OptT (Set only) and NilT (Null only) return
// false.
func isOptionalNullable(t *ir.Type) bool {
	if t == nil {
		return false
	}
	switch t.Kind {
	case ir.KindGeneric:
		return t.GenericVariant.NullableOptional()
	case ir.KindAlias:
		if t.AliasTo != nil {
			return isOptionalNullable(t.AliasTo)
		}
		return false
	default:
		return false
	}
}

// categorizeFieldType maps an IR type to a FieldTypeCategory
func categorizeFieldType(t *ir.Type) ir.FieldTypeCategory {
	if t == nil {
		return ir.FieldTypePrimitive
	}

	switch t.Kind {
	case ir.KindPrimitive:
		// Check for optional/nullable wrappers
		switch t.Primitive {
		case ir.String, ir.Int, ir.Int8, ir.Int16, ir.Int32, ir.Int64,
			ir.Uint, ir.Uint8, ir.Uint16, ir.Uint32, ir.Uint64,
			ir.Float32, ir.Float64, ir.Bool, ir.ByteSlice, ir.Duration, ir.Time, ir.UUID, ir.URL:
			return ir.FieldTypePrimitive
		default:
			return ir.FieldTypePrimitive
		}

	case ir.KindGeneric:
		// Generic types like OptT, NilT - classify by the wrapper variant
		if t.GenericVariant.Optional {
			return ir.FieldTypeOptional
		}
		if t.GenericVariant.Nullable {
			return ir.FieldTypeNullable
		}
		// Other generic types - check the underlying type
		if t.GenericOf != nil {
			return categorizeFieldType(t.GenericOf)
		}
		return ir.FieldTypePrimitive

	case ir.KindStruct:
		// Every plain struct is a nested object. Wrappers are KindGeneric,
		// so user-defined structs named Opt* or Nil* stay ordinary objects
		return ir.FieldTypeNestedObject

	case ir.KindArray:
		return ir.FieldTypeArray

	case ir.KindMap:
		return ir.FieldTypeMap

	case ir.KindPointer:
		return ir.FieldTypePointer

	case ir.KindAlias:
		// For aliases, check the underlying type
		if t.AliasTo != nil {
			return categorizeFieldType(t.AliasTo)
		}
		return ir.FieldTypePrimitive

	default:
		return ir.FieldTypePrimitive
	}
}
