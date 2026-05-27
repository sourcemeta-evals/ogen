package parser

import (
	"testing"

	"github.com/go-faster/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ogen-go/ogen"
	"github.com/ogen-go/ogen/jsonschema"
	"github.com/ogen-go/ogen/openapi"
)

// Check that parser correctly handles paths with escaped slashes.
func TestEscapedSlashInPath(t *testing.T) {
	// If you unescape "/user%2Fget", it will be "/user/get".
	//
	// But this slash is not the path separator, it's a part of the path.
	//
	// Parser should keep it as is, since these paths are two different paths.
	root := &ogen.Spec{
		OpenAPI: "3.0.3",
		Paths: map[string]*ogen.PathItem{
			"/user/get": {
				Get: &ogen.Operation{
					OperationID: "userGet",
					Description: "operation description",
					Responses: map[string]*ogen.Response{
						"200": {},
					},
				},
			},
			"/user%2Fget": {
				Get: &ogen.Operation{
					OperationID: "escapedUserGet",
					Description: "operation description",
					Responses: map[string]*ogen.Response{
						"200": {},
					},
				},
			},
		},
	}

	a := require.New(t)

	var raw yaml.Node
	a.NoError(raw.Encode(root))
	root.Raw = &raw

	spec, err := Parse(root, Settings{
		RootURL: testRootURL,
	})
	a.NoError(err)
	a.Len(spec.Operations, 2)
}

func TestXOgenOperationGroup(t *testing.T) {
	root := &ogen.Spec{
		OpenAPI: "3.0.3",
		Paths: map[string]*ogen.PathItem{
			"/users": {
				Common: extensionValue(xOgenOperationGroup, "Users"),
				Get: &ogen.Operation{
					OperationID: "userList",
					Responses: map[string]*ogen.Response{
						"200": {},
					},
				},
				Post: &ogen.Operation{
					Common:      extensionValue(xOgenOperationGroup, "Override"),
					OperationID: "userCreate",
					Responses: map[string]*ogen.Response{
						"200": {},
					},
				},
			},
		},
	}

	spec, err := Parse(root, Settings{
		RootURL: testRootURL,
	})
	assert.NoError(t, err)

	expected := &openapi.API{
		Version: openapi.Version{Major: 3, Minor: 0, Patch: 3},
		Tags:    []openapi.Tag{},
		Operations: []*openapi.Operation{
			{
				OperationID: "userList",
				HTTPMethod:  "get",
				Path: openapi.Path{
					{Raw: "/users"},
				},
				Parameters: []*openapi.Parameter{},
				Security:   openapi.SecurityRequirements{},
				Responses: openapi.Responses{
					StatusCode: map[int]*openapi.Response{
						200: {},
					},
				},
				XOgenOperationGroup: "Users",
			},
			{
				OperationID: "userCreate",
				HTTPMethod:  "post",
				Path: openapi.Path{
					{Raw: "/users"},
				},
				Parameters: []*openapi.Parameter{},
				Security:   openapi.SecurityRequirements{},
				Responses: openapi.Responses{
					StatusCode: map[int]*openapi.Response{
						200: {},
					},
				},
				XOgenOperationGroup: "Override",
			},
		},
		Components: &openapi.Components{
			Schemas:       map[string]*jsonschema.Schema{},
			Responses:     map[string]*openapi.Response{},
			Parameters:    map[string]*openapi.Parameter{},
			Examples:      map[string]*openapi.Example{},
			RequestBodies: map[string]*openapi.RequestBody{},
		},
	}

	assert.Equal(t, expected, spec)
}

func extensionValue(name, value string) ogen.OpenAPICommon {
	return ogen.OpenAPICommon{
		Extensions: ogen.Extensions{
			name: yaml.Node{Kind: yaml.ScalarNode, Value: value},
		},
	}
}

// TestDuplicatePathsDifferentMethods tests that paths with the same structure
// but different parameter names are allowed when they have different HTTP methods.
func TestDuplicatePathsDifferentMethods(t *testing.T) {
	root := &ogen.Spec{
		OpenAPI: "3.0.3",
		Paths: map[string]*ogen.PathItem{
			"/pets/{petId}": {
				Get: &ogen.Operation{
					OperationID: "getPet",
					Parameters: []*ogen.Parameter{
						{
							Name:     "petId",
							In:       "path",
							Required: true,
							Schema:   &ogen.Schema{Type: "string"},
						},
					},
					Responses: map[string]*ogen.Response{
						"200": {Description: "OK"},
					},
				},
			},
			"/pets/{id}": {
				Post: &ogen.Operation{
					OperationID: "createPet",
					Parameters: []*ogen.Parameter{
						{
							Name:     "id",
							In:       "path",
							Required: true,
							Schema:   &ogen.Schema{Type: "string"},
						},
					},
					Responses: map[string]*ogen.Response{
						"200": {Description: "OK"},
					},
				},
			},
		},
	}

	a := require.New(t)

	var raw yaml.Node
	a.NoError(raw.Encode(root))
	root.Raw = &raw

	// Default behavior: should allow duplicate paths with different methods
	spec, err := Parse(root, Settings{
		RootURL: testRootURL,
	})
	a.NoError(err)
	a.Len(spec.Operations, 2)

	// Verify both operations are present with correct paths
	var foundGet, foundPost bool
	for _, op := range spec.Operations {
		switch op.OperationID {
		case "getPet":
			foundGet = true
			a.Equal("get", op.HTTPMethod)
			a.Equal("/pets/{petId}", op.Path.String())
		case "createPet":
			foundPost = true
			a.Equal("post", op.HTTPMethod)
			a.Equal("/pets/{id}", op.Path.String())
		}
	}
	a.True(foundGet, "GET operation not found")
	a.True(foundPost, "POST operation not found")
}

// TestDuplicatePathsDifferentMethodsDisabled tests that paths with the same structure
// but different parameter names are rejected when DisallowDuplicateMethodPaths is true.
func TestDuplicatePathsDifferentMethodsDisabled(t *testing.T) {
	root := &ogen.Spec{
		OpenAPI: "3.0.3",
		Paths: map[string]*ogen.PathItem{
			"/pets/{petId}": {
				Get: &ogen.Operation{
					OperationID: "getPet",
					Parameters: []*ogen.Parameter{
						{
							Name:     "petId",
							In:       "path",
							Required: true,
							Schema:   &ogen.Schema{Type: "string"},
						},
					},
					Responses: map[string]*ogen.Response{
						"200": {Description: "OK"},
					},
				},
			},
			"/pets/{id}": {
				Post: &ogen.Operation{
					OperationID: "createPet",
					Parameters: []*ogen.Parameter{
						{
							Name:     "id",
							In:       "path",
							Required: true,
							Schema:   &ogen.Schema{Type: "string"},
						},
					},
					Responses: map[string]*ogen.Response{
						"200": {Description: "OK"},
					},
				},
			},
		},
	}

	a := require.New(t)

	var raw yaml.Node
	a.NoError(raw.Encode(root))
	root.Raw = &raw

	// With strict mode: should reject duplicate paths even with different methods
	_, err := Parse(root, Settings{
		RootURL:                      testRootURL,
		DisallowDuplicateMethodPaths: true,
	})
	a.Error(err)
	a.Contains(err.Error(), "duplicate path")
}

// TestDuplicatePathsSameMethod tests that paths with the same structure,
// same HTTP method, but different parameter names are always rejected.
func TestDuplicatePathsSameMethod(t *testing.T) {
	root := &ogen.Spec{
		OpenAPI: "3.0.3",
		Paths: map[string]*ogen.PathItem{
			"/pets/{petId}": {
				Get: &ogen.Operation{
					OperationID: "getPetById",
					Parameters: []*ogen.Parameter{
						{
							Name:     "petId",
							In:       "path",
							Required: true,
							Schema:   &ogen.Schema{Type: "string"},
						},
					},
					Responses: map[string]*ogen.Response{
						"200": {Description: "OK"},
					},
				},
			},
			"/pets/{id}": {
				Get: &ogen.Operation{
					OperationID: "getPet",
					Parameters: []*ogen.Parameter{
						{
							Name:     "id",
							In:       "path",
							Required: true,
							Schema:   &ogen.Schema{Type: "string"},
						},
					},
					Responses: map[string]*ogen.Response{
						"200": {Description: "OK"},
					},
				},
			},
		},
	}

	a := require.New(t)

	var raw yaml.Node
	a.NoError(raw.Encode(root))
	root.Raw = &raw

	// Same method on duplicate paths should always error
	_, err := Parse(root, Settings{
		RootURL: testRootURL,
	})
	a.Error(err)
	a.Contains(err.Error(), "duplicate path")
}

// TestPathItemSharedParametersOnNewMethods verifies that PathItem-level
// parameters are inherited by `query` and `additionalOperations` operations,
// which relies on `getPathMethods` correctly enumerating the new methods
// (including lowercasing of custom-method keys for downstream comparisons).
func TestPathItemSharedParametersOnNewMethods(t *testing.T) {
	root := &ogen.Spec{
		OpenAPI: "3.2.0",
		Paths: map[string]*ogen.PathItem{
			"/resource/{id}": {
				Parameters: []*ogen.Parameter{
					{
						Name:     "id",
						In:       "path",
						Required: true,
						Schema:   &ogen.Schema{Type: "string"},
					},
				},
				Query: &ogen.Operation{
					OperationID: "queryResource",
					Responses: map[string]*ogen.Response{
						"200": {Description: "OK"},
					},
				},
				AdditionalOperations: map[string]*ogen.Operation{
					"LINK": {
						OperationID: "linkResource",
						Responses: map[string]*ogen.Response{
							"200": {Description: "OK"},
						},
					},
					"Notify": {
						OperationID: "notifyResource",
						Responses: map[string]*ogen.Response{
							"200": {Description: "OK"},
						},
					},
				},
			},
		},
	}

	a := require.New(t)

	var raw yaml.Node
	a.NoError(raw.Encode(root))
	root.Raw = &raw

	spec, err := Parse(root, Settings{
		RootURL: testRootURL,
	})
	a.NoError(err)

	// Three operations: Query + LINK + Notify, all on /resource/{id}.
	a.Len(spec.Operations, 3)

	byID := make(map[string]*openapi.Operation, len(spec.Operations))
	for _, op := range spec.Operations {
		byID[op.OperationID] = op
	}
	a.Contains(byID, "queryResource")
	a.Contains(byID, "linkResource")
	a.Contains(byID, "notifyResource")

	// Each operation must inherit the PathItem-level `id` path parameter.
	for _, opID := range []string{"queryResource", "linkResource", "notifyResource"} {
		op := byID[opID]
		a.NotNil(op, "operation %q must be parsed", opID)
		a.Equal("/resource/{id}", op.Path.String(), "operation %q on /resource/{id}", opID)
		var found bool
		for _, param := range op.Parameters {
			if param.Name == "id" && param.In == openapi.LocationPath {
				found = true
				break
			}
		}
		a.True(found, "operation %q must inherit the PathItem `id` path parameter", opID)
	}
}

// TestPathItemDuplicateCustomMethodAcrossNormalizedPaths verifies that
// `getPathMethods` lowercases `additionalOperations` keys: two paths with
// the same normalized shape and the same custom method spelled with
// different capitalization must be detected as a duplicate-path conflict.
func TestPathItemDuplicateCustomMethodAcrossNormalizedPaths(t *testing.T) {
	root := &ogen.Spec{
		OpenAPI: "3.2.0",
		Paths: map[string]*ogen.PathItem{
			"/resource/{a}": {
				Parameters: []*ogen.Parameter{
					{Name: "a", In: "path", Required: true, Schema: &ogen.Schema{Type: "string"}},
				},
				AdditionalOperations: map[string]*ogen.Operation{
					"LINK": {
						OperationID: "linkResourceA",
						Responses: map[string]*ogen.Response{
							"200": {Description: "OK"},
						},
					},
				},
			},
			"/resource/{b}": {
				Parameters: []*ogen.Parameter{
					{Name: "b", In: "path", Required: true, Schema: &ogen.Schema{Type: "string"}},
				},
				AdditionalOperations: map[string]*ogen.Operation{
					"link": {
						OperationID: "linkResourceB",
						Responses: map[string]*ogen.Response{
							"200": {Description: "OK"},
						},
					},
				},
			},
		},
	}

	a := require.New(t)

	var raw yaml.Node
	a.NoError(raw.Encode(root))
	root.Raw = &raw

	_, err := Parse(root, Settings{
		RootURL: testRootURL,
	})
	a.Error(err)
	a.Contains(err.Error(), "duplicate path")
}
