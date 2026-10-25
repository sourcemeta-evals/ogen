package gen

import (
	"fmt"
	"testing"

	"github.com/go-faster/yaml"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/ogen-go/ogen/gen/ir"
	"github.com/ogen-go/ogen/jsonpointer"
	"github.com/ogen-go/ogen/jsonschema"
	"github.com/ogen-go/ogen/location"
)

func TestSchemaGenAnyWarn(t *testing.T) {
	a := require.New(t)

	core, ob := observer.New(zap.InfoLevel)
	s := newSchemaGen(func(ref jsonschema.Ref) (*ir.Type, bool) {
		return nil, false
	})
	s.log = zap.New(core)

	_, err := s.generate("foo", &jsonschema.Schema{
		Type: "",
	}, false)
	a.NoError(err)

	entries := ob.FilterMessage("Type is not defined, using any").All()
	a.Len(entries, 1)
	args := entries[0].ContextMap()
	a.Equal("foo", args["name"])
}

func TestGenerate(t *testing.T) {
	var loc location.Locator
	loc.UnmarshalYAML(&yaml.Node{
		Line:   1,
		Column: 1,
	})
	pointer := location.Pointer{
		Source: location.File{
			Name:   "pet-tags.yml",
			Source: "pet-tags.yml",
		},
		Locator: loc,
	}

	tests := []struct {
		name           string
		schema         *jsonschema.Schema
		optional       bool
		expectedIrType func(*jsonschema.Schema) *ir.Type
		expectedErr    string
	}{
		{
			name: "fieldTag",
			schema: &jsonschema.Schema{
				Ref: jsonpointer.RefKey{
					Loc: "jsonschema://dummy",
					Ptr: "#/components/schemas/Pet",
				},
				Type: "object",
				Properties: []jsonschema.Property{
					{
						Name: "id",
						Schema: &jsonschema.Schema{
							Type:   "integer",
							Format: "int64",
							ExtraTags: map[string]string{
								"gorm":  "primaryKey",
								"valid": "customIdValidator",
							},
							Pointer: pointer,
						},
						Required: true,
					},

					{
						Name: "name",
						Schema: &jsonschema.Schema{
							Type: "string",
							ExtraTags: map[string]string{
								"valid": "customNameValidator",
							},
							Pointer: pointer,
						},
						Required: true,
					},

					{
						Name: "tag",
						Schema: &jsonschema.Schema{
							Type:    "string",
							Pointer: pointer,
						},
						Required: true,
					},
				},
			},
			optional:    false,
			expectedErr: "",
			expectedIrType: func(schema *jsonschema.Schema) *ir.Type {
				return &ir.Type{
					Kind: "struct",
					Name: "Pet",
					Fields: []*ir.Field{
						{
							Name: "ID",
							Type: &ir.Type{
								Kind:      "primitive",
								Primitive: ir.Int64,
								Schema:    schema.Properties[0].Schema,
							},
							Tag: ir.Tag{
								JSON: "id",
								ExtraTags: map[string]string{
									"gorm":  "primaryKey",
									"valid": "customIdValidator",
								},
							},
							Spec: &jsonschema.Property{
								Name:     "id",
								Schema:   schema.Properties[0].Schema,
								Required: true,
							},
						},

						{
							Name: "Name",
							Type: &ir.Type{
								Kind:      "primitive",
								Primitive: ir.String,
								Schema:    schema.Properties[1].Schema,
							},
							Tag: ir.Tag{
								JSON: "name",
								ExtraTags: map[string]string{
									"valid": "customNameValidator",
								},
							},
							Spec: &jsonschema.Property{
								Name:     "name",
								Schema:   schema.Properties[1].Schema,
								Required: true,
							},
						},

						{
							Name: "Tag",
							Type: &ir.Type{
								Kind:      "primitive",
								Primitive: ir.String,
								Schema:    schema.Properties[2].Schema,
							},
							Tag: ir.Tag{
								JSON: "tag",
							},
							Spec: &jsonschema.Property{
								Name:     "tag",
								Schema:   schema.Properties[2].Schema,
								Required: true,
							},
						},
					},
					Schema: schema,
				}
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(fmt.Sprintf("Test %s", tt.name), func(t *testing.T) {
			a := require.New(t)

			core, _ := observer.New(zap.InfoLevel)
			s := newSchemaGen(func(ref jsonschema.Ref) (*ir.Type, bool) {
				return nil, false
			})
			s.log = zap.New(core)

			irType, err := s.generate(tt.name, tt.schema, tt.optional)

			var errText string
			if err != nil {
				errText = err.Error()
			}
			a.Equal(tt.expectedErr, errText, "err")

			expectedIrType := tt.expectedIrType(tt.schema)
			expectedIrTypeY, _ := yaml.Marshal(expectedIrType)
			irTypeY, _ := yaml.Marshal(irType)
			a.Equal(expectedIrType, irType, fmt.Sprintf("\nEXPECTED:\n\n%s\nACTUAL:\n\n%s", expectedIrTypeY, irTypeY))
		})
	}
}

func TestSchemaGenUniqueItemsAcceptsComplexItems(t *testing.T) {
	objectWithProperties := &jsonschema.Schema{
		Type: jsonschema.Object,
		Properties: []jsonschema.Property{
			{
				Name:   "id",
				Schema: &jsonschema.Schema{Type: jsonschema.String},
			},
		},
	}

	tests := []struct {
		name string
		item *jsonschema.Schema
	}{
		{
			name: "string items",
			item: &jsonschema.Schema{Type: jsonschema.String},
		},
		{
			name: "object items with properties",
			item: objectWithProperties,
		},
		{
			name: "missing item type",
			item: &jsonschema.Schema{Type: ""},
		},
		{
			name: "array items",
			item: &jsonschema.Schema{
				Type: jsonschema.Array,
				Item: &jsonschema.Schema{Type: jsonschema.String},
			},
		},
		{
			name: "nested array of object items",
			item: &jsonschema.Schema{
				Type: jsonschema.Array,
				Item: objectWithProperties,
			},
		},
		{
			name: "object items without properties",
			item: func() *jsonschema.Schema {
				additional := true
				return &jsonschema.Schema{
					Type:                 jsonschema.Object,
					AdditionalProperties: &additional,
					Item:                 &jsonschema.Schema{Type: jsonschema.String},
				}
			}(),
		},
		{
			name: "nullable object items",
			item: &jsonschema.Schema{
				Type:     jsonschema.Object,
				Nullable: true,
				Properties: []jsonschema.Property{
					{
						Name:   "id",
						Schema: &jsonschema.Schema{Type: jsonschema.String},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := require.New(t)
			s := newSchemaGen(func(ref jsonschema.Ref) (*ir.Type, bool) {
				return nil, false
			})

			_, err := s.generate("Thing", &jsonschema.Schema{
				Type:        jsonschema.Array,
				Item:        tt.item,
				UniqueItems: true,
			}, false)
			a.NoError(err)
		})
	}
}

func TestSchemaGenTupleUniqueItemsRejected(t *testing.T) {
	a := require.New(t)
	tuple := []*jsonschema.Schema{
		{Type: jsonschema.String},
		{Type: jsonschema.String},
	}

	s := newSchemaGen(func(ref jsonschema.Ref) (*ir.Type, bool) {
		return nil, false
	})
	_, err := s.generate("Pair", &jsonschema.Schema{
		Type:        jsonschema.Array,
		Items:       tuple,
		UniqueItems: true,
	}, false)
	var notImplemented *ErrNotImplemented
	a.ErrorAs(err, &notImplemented)
	a.Equal("tuple uniqueItems", notImplemented.Name)

	s = newSchemaGen(func(ref jsonschema.Ref) (*ir.Type, bool) {
		return nil, false
	})
	_, err = s.generate("Pair", &jsonschema.Schema{
		Type:  jsonschema.Array,
		Items: tuple,
	}, false)
	a.NoError(err)
}

func TestSchemaGenAllOfTupleUniqueItemsRejected(t *testing.T) {
	a := require.New(t)
	makeTuple := func() *jsonschema.Schema {
		return &jsonschema.Schema{
			Type: jsonschema.Array,
			Items: []*jsonschema.Schema{
				{Type: jsonschema.String},
				{Type: jsonschema.String},
			},
			UniqueItems: true,
		}
	}

	// Merging allOf branches must not discard tuple items or uniqueness:
	// the merged schema has to reach the tuple uniqueItems rejection
	s := newSchemaGen(func(ref jsonschema.Ref) (*ir.Type, bool) {
		return nil, false
	})
	_, err := s.generate("Pair", &jsonschema.Schema{
		AllOf:       []*jsonschema.Schema{makeTuple(), makeTuple()},
		UniqueItems: true,
	}, false)
	var notImplemented *ErrNotImplemented
	a.ErrorAs(err, &notImplemented)
	a.Equal("tuple uniqueItems", notImplemented.Name)
}

func TestSchemaGenSumOuterUniqueItemsPropagated(t *testing.T) {
	arrayVariant := func() *jsonschema.Schema {
		return &jsonschema.Schema{
			Type:     jsonschema.Array,
			MinItems: func() *uint64 { n := uint64(1); return &n }(),
			Item: &jsonschema.Schema{
				Type: jsonschema.Object,
				Properties: []jsonschema.Property{
					{Name: "name", Schema: &jsonschema.Schema{Type: jsonschema.String}, Required: true},
					{Name: "value", Schema: &jsonschema.Schema{Type: jsonschema.String}, Required: true},
				},
			},
		}
	}
	stringVariant := func() *jsonschema.Schema {
		return &jsonschema.Schema{Type: jsonschema.String}
	}

	for _, tt := range []struct {
		name   string
		schema *jsonschema.Schema
	}{
		{
			name: "anyOf",
			schema: &jsonschema.Schema{
				AnyOf:       []*jsonschema.Schema{arrayVariant(), stringVariant()},
				UniqueItems: true,
			},
		},
		{
			name: "oneOf",
			schema: &jsonschema.Schema{
				OneOf:       []*jsonschema.Schema{arrayVariant(), stringVariant()},
				UniqueItems: true,
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a := require.New(t)
			s := newSchemaGen(func(ref jsonschema.Ref) (*ir.Type, bool) {
				return nil, false
			})
			sum, err := s.generate("Flags", tt.schema, false)
			a.NoError(err)
			a.Equal(ir.KindSum, sum.Kind)

			var arrays int
			for _, variant := range sum.SumOf {
				if variant.Kind != ir.KindArray {
					continue
				}
				arrays++
				// The variant's own length validators must not block the
				// outer uniqueness constraint from reaching the variant
				a.True(variant.Validators.Array.UniqueItems,
					"outer uniqueItems must reach the array variant")
				a.True(variant.Validators.Array.MinLengthSet)
			}
			a.Equal(1, arrays)
		})
	}
}
