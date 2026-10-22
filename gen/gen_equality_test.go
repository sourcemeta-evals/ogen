package gen

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ogen-go/ogen/gen/ir"
	"github.com/ogen-go/ogen/jsonschema"
)

func TestCreateEqualityMethodSpec_ArrayDetection(t *testing.T) {
	tests := []struct {
		name           string
		irType         *ir.Type
		expectedFields map[string]fieldAssertions
	}{
		{
			name: "optional array field",
			irType: &ir.Type{
				Name: "TestType",
				Kind: ir.KindStruct,
				Fields: []*ir.Field{
					{
						Name: "OptionalArray",
						Type: &ir.Type{
							Name: "OptStringArray",
							Kind: ir.KindGeneric,
							GenericOf: &ir.Type{
								Kind: ir.KindArray,
								Item: &ir.Type{
									Kind:      ir.KindPrimitive,
									Primitive: ir.String,
								},
							},
						},
					},
				},
			},
			expectedFields: map[string]fieldAssertions{
				"OptionalArray": {
					fieldType: ir.FieldTypeOptional,
					isArray:   true,
				},
			},
		},
		{
			name: "nullable array field",
			irType: &ir.Type{
				Name: "TestType",
				Kind: ir.KindStruct,
				Fields: []*ir.Field{
					{
						Name: "NullableArray",
						Type: &ir.Type{
							Name: "NilIntArray",
							Kind: ir.KindGeneric,
							GenericOf: &ir.Type{
								Kind: ir.KindArray,
								Item: &ir.Type{
									Kind:      ir.KindPrimitive,
									Primitive: ir.Int,
								},
							},
						},
					},
				},
			},
			expectedFields: map[string]fieldAssertions{
				"NullableArray": {
					fieldType: ir.FieldTypeNullable,
					isArray:   true,
				},
			},
		},
		{
			name: "direct array field",
			irType: &ir.Type{
				Name: "TestType",
				Kind: ir.KindStruct,
				Fields: []*ir.Field{
					{
						Name: "DirectArray",
						Type: &ir.Type{
							Kind: ir.KindArray,
							Item: &ir.Type{
								Kind:      ir.KindPrimitive,
								Primitive: ir.String,
							},
						},
					},
				},
			},
			expectedFields: map[string]fieldAssertions{
				"DirectArray": {
					fieldType: ir.FieldTypeArray,
					isArray:   true,
				},
			},
		},
		{
			name: "optional array of structs",
			irType: &ir.Type{
				Name: "TestType",
				Kind: ir.KindStruct,
				Fields: []*ir.Field{
					{
						Name: "OptionalStructArray",
						Type: &ir.Type{
							Name: "OptStructArray",
							Kind: ir.KindGeneric,
							GenericOf: &ir.Type{
								Kind: ir.KindArray,
								Item: &ir.Type{
									Name: "NestedStruct",
									Kind: ir.KindStruct,
								},
							},
						},
					},
				},
			},
			expectedFields: map[string]fieldAssertions{
				"OptionalStructArray": {
					fieldType:        ir.FieldTypeOptional,
					isArray:          true,
					isArrayOfStructs: true,
				},
			},
		},
		{
			name: "optional map field (not array)",
			irType: &ir.Type{
				Name: "TestType",
				Kind: ir.KindStruct,
				Fields: []*ir.Field{
					{
						Name: "OptionalMap",
						Type: &ir.Type{
							Name: "OptStringMap",
							Kind: ir.KindGeneric,
							GenericOf: &ir.Type{
								Kind: ir.KindMap,
							},
						},
					},
				},
			},
			expectedFields: map[string]fieldAssertions{
				"OptionalMap": {
					fieldType: ir.FieldTypeOptional,
					isMap:     true,
					isArray:   false,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &Generator{
				equalitySpecs: []*ir.EqualityMethodSpec{},
			}

			spec := g.createEqualityMethodSpec(tt.irType)
			require.NotNil(t, spec, "spec should not be nil")
			require.Equal(t, tt.irType.Name, spec.TypeName)

			for fieldName, assertions := range tt.expectedFields {
				var found *ir.FieldEqualitySpec
				for i := range spec.Fields {
					if spec.Fields[i].FieldName == fieldName {
						found = &spec.Fields[i]
						break
					}
				}

				require.NotNil(t, found, "field %s should exist in spec", fieldName)
				require.Equal(t, assertions.fieldType, found.FieldType,
					"field %s FieldType mismatch", fieldName)
				require.Equal(t, assertions.isArray, found.IsArray,
					"field %s IsArray mismatch", fieldName)
				require.Equal(t, assertions.isArrayOfStructs, found.IsArrayOfStructs,
					"field %s IsArrayOfStructs mismatch", fieldName)
				require.Equal(t, assertions.isMap, found.IsMap,
					"field %s IsMap mismatch", fieldName)
			}
		})
	}
}

type fieldAssertions struct {
	fieldType        ir.FieldTypeCategory
	isArray          bool
	isArrayOfStructs bool
	isMap            bool
}

func TestWriteArrayComparison(t *testing.T) {
	tests := []struct {
		name             string
		aArray           string
		bArray           string
		indent           string
		isArrayOfStructs bool
		hasDepth         bool
		expectedContains []string
	}{
		{
			name:             "primitive array without depth",
			aArray:           "a.Items",
			bArray:           "b.Items",
			indent:           "\t",
			isArrayOfStructs: false,
			hasDepth:         false,
			expectedContains: []string{
				"if len(a.Items) != len(b.Items)",
				"for i := range a.Items",
				"if a.Items[i] != b.Items[i]",
				"return false",
			},
		},
		{
			name:             "struct array with depth",
			aArray:           "a.Nested",
			bArray:           "b.Nested",
			indent:           "\t\t",
			isArrayOfStructs: true,
			hasDepth:         true,
			expectedContains: []string{
				"if len(a.Nested) != len(b.Nested)",
				"for i := range a.Nested",
				"if !a.Nested[i].Equal(b.Nested[i], depth+1)",
				"return false",
			},
		},
		{
			name:             "struct array without depth",
			aArray:           "a.Nested",
			bArray:           "b.Nested",
			indent:           "\t",
			isArrayOfStructs: true,
			hasDepth:         false,
			expectedContains: []string{
				"if len(a.Nested) != len(b.Nested)",
				"for i := range a.Nested",
				"if !a.Nested[i].Equal(b.Nested[i])",
				"return false",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			g := &Generator{}

			g.writeArrayComparison(&b, tt.aArray, tt.bArray, tt.indent, tt.isArrayOfStructs, tt.hasDepth)

			output := b.String()
			for _, expected := range tt.expectedContains {
				require.Contains(t, output, expected,
					"output should contain: %s", expected)
			}

			// Verify proper indentation
			lines := strings.Split(output, "\n")
			for _, line := range lines {
				if line != "" {
					require.True(t, strings.HasPrefix(line, tt.indent),
						"line should start with indent %q: %s", tt.indent, line)
				}
			}
		})
	}
}

func TestWriteFieldComparison_OptionalArray(t *testing.T) {
	tests := []struct {
		name             string
		field            ir.FieldEqualitySpec
		hasDepth         bool
		expectedContains []string
		notContains      []string
	}{
		{
			name: "optional primitive array",
			field: ir.FieldEqualitySpec{
				FieldName:        "Items",
				FieldType:        ir.FieldTypeOptional,
				IsArray:          true,
				IsArrayOfStructs: false,
			},
			hasDepth: false,
			expectedContains: []string{
				"// Compare optional field: Items",
				"if a.Items.Set != b.Items.Set",
				"if a.Items.Set {",
				"if len(a.Items.Value) != len(b.Items.Value)",
				"for i := range a.Items.Value",
				"if a.Items.Value[i] != b.Items.Value[i]",
			},
			notContains: []string{
				"if a.Items.Value != b.Items.Value", // Should NOT use direct slice comparison
			},
		},
		{
			name: "optional struct array with depth",
			field: ir.FieldEqualitySpec{
				FieldName:        "Nested",
				FieldType:        ir.FieldTypeOptional,
				IsArray:          true,
				IsArrayOfStructs: true,
			},
			hasDepth: true,
			expectedContains: []string{
				"// Compare optional field: Nested",
				"if a.Nested.Set != b.Nested.Set",
				"if a.Nested.Set {",
				"if len(a.Nested.Value) != len(b.Nested.Value)",
				"for i := range a.Nested.Value",
				"if !a.Nested.Value[i].Equal(b.Nested.Value[i], depth+1)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			g := &Generator{}

			g.writeFieldComparison(&b, tt.field, tt.hasDepth)

			output := b.String()
			for _, expected := range tt.expectedContains {
				require.Contains(t, output, expected,
					"output should contain: %s", expected)
			}
			for _, notExpected := range tt.notContains {
				require.NotContains(t, output, notExpected,
					"output should NOT contain: %s", notExpected)
			}
		})
	}
}

func TestWriteFieldComparison_OptionalNullable(t *testing.T) {
	tests := []struct {
		name             string
		field            ir.FieldEqualitySpec
		hasDepth         bool
		expectedContains []string
		notContains      []string
	}{
		{
			name: "optional-nullable primitive",
			field: ir.FieldEqualitySpec{
				FieldName:          "Name",
				FieldType:          ir.FieldTypeOptional,
				IsOptionalNullable: true,
			},
			hasDepth: false,
			expectedContains: []string{
				"// Compare optional field: Name",
				"if a.Name.Set != b.Name.Set",
				"if a.Name.Set {",
				"if a.Name.Null != b.Name.Null",
				"if !a.Name.Null {",
				"if a.Name.Value != b.Name.Value",
			},
		},
		{
			name: "optional-nullable nested struct with depth",
			field: ir.FieldEqualitySpec{
				FieldName:          "Nested",
				FieldType:          ir.FieldTypeOptional,
				IsOptionalNullable: true,
				IsNested:           true,
			},
			hasDepth: true,
			expectedContains: []string{
				"// Compare optional field: Nested",
				"if a.Nested.Set != b.Nested.Set",
				"if a.Nested.Set {",
				"if a.Nested.Null != b.Nested.Null",
				"if !a.Nested.Null {",
				"if !a.Nested.Value.Equal(b.Nested.Value, depth+1)",
			},
		},
		{
			name: "plain optional (not nullable) omits Null check",
			field: ir.FieldEqualitySpec{
				FieldName:          "Age",
				FieldType:          ir.FieldTypeOptional,
				IsOptionalNullable: false,
			},
			hasDepth: false,
			expectedContains: []string{
				"// Compare optional field: Age",
				"if a.Age.Set != b.Age.Set",
				"if a.Age.Set {",
				"if a.Age.Value != b.Age.Value",
			},
			notContains: []string{
				"if a.Age.Null != b.Age.Null",
				"if !a.Age.Null {",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			g := &Generator{}

			g.writeFieldComparison(&b, tt.field, tt.hasDepth)

			output := b.String()
			for _, expected := range tt.expectedContains {
				require.Contains(t, output, expected,
					"output should contain: %s", expected)
			}
			for _, notExpected := range tt.notContains {
				require.NotContains(t, output, notExpected,
					"output should NOT contain: %s", notExpected)
			}
		})
	}
}

func TestWriteFieldComparison_NullableArray(t *testing.T) {
	tests := []struct {
		name             string
		field            ir.FieldEqualitySpec
		hasDepth         bool
		expectedContains []string
		notContains      []string
	}{
		{
			name: "nullable primitive array",
			field: ir.FieldEqualitySpec{
				FieldName:        "Items",
				FieldType:        ir.FieldTypeNullable,
				IsArray:          true,
				IsArrayOfStructs: false,
			},
			hasDepth: false,
			expectedContains: []string{
				"// Compare nullable field: Items",
				"if a.Items.Null != b.Items.Null",
				"if !a.Items.Null {",
				"if len(a.Items.Value) != len(b.Items.Value)",
				"for i := range a.Items.Value",
				"if a.Items.Value[i] != b.Items.Value[i]",
			},
			notContains: []string{
				"if a.Items.Value != b.Items.Value", // Should NOT use direct slice comparison
			},
		},
		{
			name: "nullable struct array with depth",
			field: ir.FieldEqualitySpec{
				FieldName:        "Nested",
				FieldType:        ir.FieldTypeNullable,
				IsArray:          true,
				IsArrayOfStructs: true,
			},
			hasDepth: true,
			expectedContains: []string{
				"// Compare nullable field: Nested",
				"if a.Nested.Null != b.Nested.Null",
				"if !a.Nested.Null {",
				"if len(a.Nested.Value) != len(b.Nested.Value)",
				"for i := range a.Nested.Value",
				"if !a.Nested.Value[i].Equal(b.Nested.Value[i], depth+1)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			g := &Generator{}

			g.writeFieldComparison(&b, tt.field, tt.hasDepth)

			output := b.String()
			for _, expected := range tt.expectedContains {
				require.Contains(t, output, expected,
					"output should contain: %s", expected)
			}
			for _, notExpected := range tt.notContains {
				require.NotContains(t, output, notExpected,
					"output should NOT contain: %s", notExpected)
			}
		})
	}
}

func TestNeedsEqualityMethod(t *testing.T) {
	tests := []struct {
		name     string
		irType   *ir.Type
		expected bool
	}{
		{
			name:     "nil type",
			irType:   nil,
			expected: false,
		},
		{
			name:     "struct needs equality",
			irType:   &ir.Type{Kind: ir.KindStruct, Name: "WorkflowStatus"},
			expected: true,
		},
		{
			name: "array needs equality",
			irType: &ir.Type{
				Kind: ir.KindArray,
				Item: &ir.Type{Kind: ir.KindPrimitive, Primitive: ir.String},
			},
			expected: true,
		},
		{
			name: "map needs equality",
			irType: &ir.Type{
				Kind: ir.KindMap,
				Name: "Attributes",
				Item: &ir.Type{Kind: ir.KindPrimitive, Primitive: ir.String},
			},
			expected: true,
		},
		{
			name:     "primitive does not need equality",
			irType:   &ir.Type{Kind: ir.KindPrimitive, Primitive: ir.String},
			expected: false,
		},
		{
			name: "alias to struct needs equality",
			irType: &ir.Type{
				Kind:    ir.KindAlias,
				Name:    "StatusAlias",
				AliasTo: &ir.Type{Kind: ir.KindStruct, Name: "WorkflowStatus"},
			},
			expected: true,
		},
		{
			name: "alias to primitive does not need equality",
			irType: &ir.Type{
				Kind:    ir.KindAlias,
				Name:    "IDAlias",
				AliasTo: &ir.Type{Kind: ir.KindPrimitive, Primitive: ir.String},
			},
			expected: false,
		},
		{
			name:     "alias without target does not need equality",
			irType:   &ir.Type{Kind: ir.KindAlias, Name: "Dangling"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, needsEqualityMethod(tt.irType))
		})
	}
}

func TestCollectEqualitySpecs(t *testing.T) {
	properties := &ir.Type{
		Kind: ir.KindStruct,
		Name: "StatusProperties",
		Fields: []*ir.Field{
			{Name: "Key", Type: &ir.Type{Kind: ir.KindPrimitive, Primitive: ir.String}},
		},
	}
	attributeValue := &ir.Type{
		Kind: ir.KindStruct,
		Name: "AttributeValue",
		Fields: []*ir.Field{
			{Name: "Raw", Type: &ir.Type{Kind: ir.KindPrimitive, Primitive: ir.String}},
		},
	}
	workflowStatus := &ir.Type{
		Kind: ir.KindStruct,
		Name: "WorkflowStatus",
		Fields: []*ir.Field{
			{Name: "ID", Type: &ir.Type{Kind: ir.KindPrimitive, Primitive: ir.String}},
			{Name: "Properties", Type: properties},
			{Name: "Attributes", Type: &ir.Type{
				Kind: ir.KindMap,
				Name: "WorkflowStatusAttributes",
				Item: attributeValue,
			}},
		},
	}
	uniqueStructArray := &ir.Type{
		Kind:   ir.KindArray,
		Item:   workflowStatus,
		Schema: &jsonschema.Schema{UniqueItems: true},
	}
	plainStatus := &ir.Type{
		Kind: ir.KindStruct,
		Name: "PlainStatus",
		Fields: []*ir.Field{
			{Name: "ID", Type: &ir.Type{Kind: ir.KindPrimitive, Primitive: ir.String}},
		},
	}
	nonUniqueStructArray := &ir.Type{
		Kind:   ir.KindArray,
		Item:   plainStatus,
		Schema: &jsonschema.Schema{UniqueItems: false},
	}
	uniquePrimitiveArray := &ir.Type{
		Kind:   ir.KindArray,
		Item:   &ir.Type{Kind: ir.KindPrimitive, Primitive: ir.String},
		Schema: &jsonschema.Schema{UniqueItems: true},
	}
	request := &ir.Type{
		Kind: ir.KindStruct,
		Name: "Request",
		Fields: []*ir.Field{
			{Name: "Statuses", Type: uniqueStructArray},
			{Name: "MoreStatuses", Type: uniqueStructArray},
			{Name: "History", Type: nonUniqueStructArray},
			{Name: "Tags", Type: uniquePrimitiveArray},
		},
	}

	g := &Generator{
		tstorage:      newTStorage(),
		equalitySpecs: []*ir.EqualityMethodSpec{},
	}
	g.tstorage.types["Request"] = request

	g.collectEqualitySpecs()

	collected := map[string]*ir.EqualityMethodSpec{}
	for _, spec := range g.equalitySpecs {
		require.NotContains(t, collected, spec.TypeName,
			"type %s should only be collected once", spec.TypeName)
		collected[spec.TypeName] = spec
	}

	// The uniqueItems array item and every struct reachable from it need methods
	require.Contains(t, collected, "WorkflowStatus")
	require.Contains(t, collected, "StatusProperties")
	require.Contains(t, collected, "AttributeValue",
		"struct map values should be collected through KindMap traversal")

	// Non-unique arrays and uniqueItems arrays of primitives need no methods
	require.NotContains(t, collected, "PlainStatus")
	require.NotContains(t, collected, "Request")
	require.Len(t, collected, 3)

	// The map of structs must be flagged for recursive comparison
	var attributesField *ir.FieldEqualitySpec
	for i := range collected["WorkflowStatus"].Fields {
		if collected["WorkflowStatus"].Fields[i].FieldName == "Attributes" {
			attributesField = &collected["WorkflowStatus"].Fields[i]
			break
		}
	}
	require.NotNil(t, attributesField)
	require.Equal(t, ir.FieldTypeMap, attributesField.FieldType)
	require.True(t, attributesField.IsMapOfStructs)
}

func TestWriteMapComparison(t *testing.T) {
	tests := []struct {
		name             string
		isMapOfStructs   bool
		hasDepth         bool
		expectedContains []string
		notContains      []string
	}{
		{
			name:           "primitive values",
			isMapOfStructs: false,
			hasDepth:       true,
			expectedContains: []string{
				"if len(a.Attrs) != len(b.Attrs)",
				"for k, v := range a.Attrs",
				"if bv, ok := b.Attrs[k]; !ok || v != bv",
			},
			notContains: []string{".Equal("},
		},
		{
			name:           "struct values with depth",
			isMapOfStructs: true,
			hasDepth:       true,
			expectedContains: []string{
				"if len(a.Attrs) != len(b.Attrs)",
				"for k, v := range a.Attrs",
				"bv, ok := b.Attrs[k]",
				"if !v.Equal(bv, depth+1)",
			},
			notContains: []string{"v != bv"},
		},
		{
			name:           "struct values without depth",
			isMapOfStructs: true,
			hasDepth:       false,
			expectedContains: []string{
				"if !v.Equal(bv)",
			},
			notContains: []string{"v != bv", "depth+1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			g := &Generator{}

			g.writeMapComparison(&b, "a.Attrs", "b.Attrs", "\t", tt.isMapOfStructs, tt.hasDepth)

			output := b.String()
			for _, expected := range tt.expectedContains {
				require.Contains(t, output, expected,
					"output should contain: %s", expected)
			}
			for _, notExpected := range tt.notContains {
				require.NotContains(t, output, notExpected,
					"output should NOT contain: %s", notExpected)
			}
		})
	}
}

func TestWriteFieldHash(t *testing.T) {
	tests := []struct {
		name             string
		field            ir.FieldHashSpec
		expectedContains []string
		notContains      []string
	}{
		{
			name: "float primitive normalizes negative zero",
			field: ir.FieldHashSpec{
				FieldName: "Score",
				FieldType: ir.FieldTypePrimitive,
				GoType:    "float64",
			},
			expectedContains: []string{
				"vScore := a.Score",
				"if vScore == 0",
				"fmt.Sprintf(\"%v\", vScore)",
			},
		},
		{
			name: "optional float normalizes negative zero",
			field: ir.FieldHashSpec{
				FieldName: "Score",
				FieldType: ir.FieldTypeOptional,
				GoType:    "float64",
			},
			expectedContains: []string{
				"if a.Score.Set",
				"vScore := a.Score.Value",
				"if vScore == 0",
			},
		},
		{
			name: "string primitive hashes directly",
			field: ir.FieldHashSpec{
				FieldName: "Name",
				FieldType: ir.FieldTypePrimitive,
				GoType:    "string",
			},
			expectedContains: []string{
				"fmt.Sprintf(\"%v\", a.Name)",
			},
			notContains: []string{"vName"},
		},
		{
			name: "struct array elements hash through their Hash method",
			field: ir.FieldHashSpec{
				FieldName:        "Watchers",
				FieldType:        ir.FieldTypeArray,
				GoType:           "[]User",
				IsArrayOfStructs: true,
			},
			expectedContains: []string{
				"for _, item := range a.Watchers",
				"binary.Write(h, binary.LittleEndian, item.Hash())",
			},
			notContains: []string{"fmt.Sprintf"},
		},
		{
			name: "nullable array elements hash null state and active value",
			field: ir.FieldHashSpec{
				FieldName:         "Values",
				FieldType:         ir.FieldTypeArray,
				GoType:            "[]NilString",
				IsArrayOfNullable: true,
			},
			expectedContains: []string{
				"if item.Null",
				"fmt.Sprintf(\"%v\", item.Value)",
			},
		},
		{
			name: "struct map values hash through their Hash method",
			field: ir.FieldHashSpec{
				FieldName:      "Attrs",
				FieldType:      ir.FieldTypeMap,
				GoType:         "AttrsMap",
				IsMapOfStructs: true,
			},
			expectedContains: []string{
				"sort.Strings(keysAttrs)",
				"binary.Write(h, binary.LittleEndian, a.Attrs[k].Hash())",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			g := &Generator{}

			g.writeFieldHash(&b, tt.field)

			output := b.String()
			for _, expected := range tt.expectedContains {
				require.Contains(t, output, expected,
					"output should contain: %s", expected)
			}
			for _, notExpected := range tt.notContains {
				require.NotContains(t, output, notExpected,
					"output should NOT contain: %s", notExpected)
			}
		})
	}
}
