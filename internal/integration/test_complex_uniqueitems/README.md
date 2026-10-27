# Complex UniqueItems Validation

This directory contains comprehensive tests and examples for complex `uniqueItems` validation support in ogen.

## Overview

Previously, ogen only supported `uniqueItems: true` on arrays of primitive types (strings, numbers, etc.). This implementation adds support for every remaining item shape: struct items get generated `Equal()` and `Hash()` methods with hash-based duplicate detection, free-form items are compared as JSON values, and the remaining shapes (nested arrays, map-only objects, nullable objects) use a reflect-based pairwise helper.

### Motivation

GitHub issue [#1563](https://github.com/ogen-go/ogen/issues/1563) identified that 11 operations in the JIRA API v3 spec were blocked because they use `uniqueItems` on arrays of complex objects. This implementation removes that limitation.

## How It Works

### 1. Equal() Method Generation

For each type used in a `uniqueItems` array, ogen generates an `Equal()` method:

```go
func (a WorkflowStatus) Equal(b WorkflowStatus, depth int) bool {
    if depth > 10 {
        panic(&validate.DepthLimitError{
            MaxDepth: 10,
            TypeName: "WorkflowStatus",
        })
    }

    // Compare all fields...
    if a.ID != b.ID {
        return false
    }
    // ... including nested objects with depth+1
    if a.Properties.Set {
        if !a.Properties.Value.Equal(b.Properties.Value, depth+1) {
            return false
        }
    }
    return true
}
```

**Features:**
- Depth tracking prevents infinite recursion on circular references
- Handles all field types: primitives, optionals, arrays, maps, nested objects
- Panics with `DepthLimitError` if depth exceeds limit (default: 10)

### 2. Hash() Method Generation

For performance, each type also gets a `Hash()` method using FNV-1a:

```go
func (a WorkflowStatus) Hash(depth int) uint64 {
    if depth > 10 {
        panic(&validate.DepthLimitError{
            MaxDepth: 10,
            TypeName: "WorkflowStatus",
        })
    }

    h := fnv.New64a()

    h.Write([]byte(fmt.Sprintf("%v", a.ID)))
    h.Write([]byte(fmt.Sprintf("%v", a.Name)))

    if a.Properties.Set {
        h.Write([]byte{1})
        nestedHash := a.Properties.Value.Hash(depth + 1)
        binary.Write(h, binary.LittleEndian, nestedHash)
    } else {
        h.Write([]byte{0})
    }

    return h.Sum64()
}
```

**Properties:**
- Equal objects **must** produce equal hashes: fields whose equality ignores
  representation details (raw JSON, reflect-compared collections, inactive
  wrapper payloads, signed zero) are excluded from the hash, and collisions
  are resolved by `Equal()`
- Fast hash computation for O(n) duplicate detection
- Enforces the same depth limit as `Equal()`, so unbounded recursion cannot
  overflow the stack before the recovery in the validator runs

### 3. Runtime Validation

ogen generates `validateUnique[TypeName]()` functions that use hash buckets:

```go
func validateUniqueWorkflowStatus(items []WorkflowStatus) (err error) {
    if len(items) <= 1 {
        return nil
    }

    // Recover from depth limit panics
    defer func() {
        if r := recover(); r != nil {
            if e, ok := r.(*validate.DepthLimitError); ok {
                err = e
            } else {
                panic(r)
            }
        }
    }()

    // Hash bucket structure
    type entry struct {
        item  WorkflowStatus
        index int
    }
    buckets := make(map[uint64][]entry, len(items))

    // O(n) duplicate detection
    for i, item := range items {
        hash := item.Hash(0)
        bucket := buckets[hash]

        // Check for duplicates in this hash bucket
        for _, existing := range bucket {
            if item.Equal(existing.item, 0) {
                return &validate.DuplicateItemsError{
                    Indices: []int{existing.index, i},
                }
            }
        }

        buckets[hash] = append(bucket, entry{item: item, index: i})
    }

    return nil
}
```

**Algorithm:**
- O(n) time complexity using hash buckets
- Hash collisions resolved by calling `Equal()`
- Returns indices of duplicate items for debugging
- Catches and returns depth limit errors

### 4. Fallback Strategies

Item shapes without generated methods still validate:

| Item shape | Strategy |
|------------|----------|
| Objects with properties | Generated `Equal()`/`Hash()` + hash buckets |
| Comparable primitives | `validate.UniqueItems` (the `==` operator) |
| Free-form values (`items: {}`) | `validate.UniqueItemsRawJSON`, semantic JSON comparison |
| Nested arrays, map-only objects, nullable objects | `validate.UniqueItemsDeep`, pairwise `reflect.DeepEqual` |

Raw JSON comparison ignores object member order, insignificant whitespace,
string escaping and numeric notation, and compares numbers exactly through
`math/big` so adjacent large integers stay distinct. The pairwise fallbacks
are O(n²) in the worst case; struct items keep the O(n) hash-bucket path.

## Examples

### Simple Schema

See `workflow-status.yaml` - demonstrates basic nested objects:

```yaml
WorkflowStatus:
  type: object
  required: [id, name]
  properties:
    id: {type: string}
    name: {type: string}
    description: {type: string}
    properties:
      $ref: '#/components/schemas/StatusProperties'
```

### Deep Nesting

See `workflow-deep.yaml` - tests 7 levels of nesting (JIRA's deepest pattern):

```yaml
WorkflowTransition → ConditionGroup → Condition → RuleConfiguration
  → ParameterGroup → Parameter → ParameterValue
```

### All Field Types

See `all-field-types.yaml` - comprehensive test with 18 field types:
- Required primitives (string, number, integer, boolean)
- Optional primitives
- Nullable fields
- Enums (required and optional)
- Arrays (primitives and nested objects)
- Maps (additionalProperties)
- Nested objects (multiple levels)

### Nested Collections and Fallbacks

See `nested-collections.yaml` - exercises every fallback item shape plus
struct items with nested collection, raw JSON and float map fields, and an
anonymous request body array of objects.

### Real-World JIRA API

See `jira-subset.yaml` - minimal JIRA API v3 subset:
- `updateWorkflowTransitionRules`: 3 uniqueItems arrays
- `updateWorkflowMapping`: uniqueItems workflow mappings

## Test Coverage

Six test suites cover the feature:

### Suite 1: Basic Tests (16 tests)
- Empty arrays
- Single elements
- Duplicate detection
- All unique items
- Nested object equality
- Hash collision handling
- Field type unit tests (T046-T052)

### Suite 2: Depth Limit Tests (6 tests)
- Direct panic at depth 11
- Fully nested object panic
- Hash consistency for deep objects
- DepthLimitError recovery
- Within-limit validation
- Duplicate detection within limit

### Suite 3: Integration Tests (8 tests)
- All field types combined
- Duplicate detection with complex objects
- Optional/Nested/Array/Map differences
- Hash/Equal consistency
- Minimal items

### Suite 4: Golden File Tests (4 tests)
- Equal() generation stability
- Hash() generation stability
- Regression prevention

### Suite 5: JIRA Subset Tests (4 tests)
- Real-world JIRA patterns
- Multiple uniqueItems arrays
- Complete request validation

### Suite 6: Nested Collections Tests
- Arrays of arrays, map-only objects, nullable objects, free-form values
- Raw JSON fields: reordered keys, whitespace, large integers
- Signed zero in float maps
- Hash coherence for inactive nullable payloads
- Anonymous operation body arrays

## Performance

Struct items validate in O(n) time through hash buckets; the pairwise
fallback helpers are O(n²) in the worst case:

| Items | Time | Benchmark |
|-------|------|-----------|
| 50 JIRA rules | ~7μs | BenchmarkJIRAWorkflowRules_50Rules |
| 100 complex items | ~0.14ms | BenchmarkValidateUniqueComprehensiveItem_100Items |
| 1,000 simple items | ~0.24ms | BenchmarkValidateUniqueWorkflowStatus_1000Items |

**40x faster** than the 10ms target for 1,000 items.

## Error Types

### DuplicateItemsError

Returned when duplicates are found:

```go
type DuplicateItemsError struct {
    Indices []int
}

// Error: "duplicate item found at indices 0 and 3"
```

### DepthLimitError

Returned when nesting exceeds the depth limit:

```go
type DepthLimitError struct {
    MaxDepth int
    TypeName string
}

// Error: "equality check depth limit exceeded for type WorkflowStatus (max: 10)"
```

## Implementation Files

### Core Generation
- `gen/ir/equality.go` - IR types for equality specs
- `gen/ir/unique_items.go` - validation strategy dispatch per item shape
- `gen/gen_equality_detect.go` - Type collection and traversal
- `gen/gen_equality.go` - Equal() and Hash() generation
- `gen/gen_validators_unique.go` - validateUnique() generation
- `gen/_template/validators.tmpl` - Integration into validation

### Validation
- `validate/errors.go` - Error types (DuplicateItemsError, DepthLimitError)
- `validate/unique_complex.go` - reflect and raw JSON fallback helpers

### Tests
- `generated/` - Basic tests with 2-level nesting
- `generated-depth-limit/` - 12-level nesting tests
- `generated-integration/` - All field types combined
- `generated-golden/` - Regression tests
- `generated-jira/` - Real-world JIRA patterns
- `generated-nested/` - Fallback item shapes and semantic field comparison

## Limitations

1. **Depth Limit**: Default maximum depth is 10, enforced by both `Equal()` and `Hash()`, so deeply nested or cyclic values return `DepthLimitError` instead of overflowing the stack
2. **Hash Collisions**: Possible by design; resolved by Equal() verification
3. **Performance**: O(n) for struct items in typical cases; the reflect and raw JSON fallback helpers compare pairwise, which is O(n²) in the array length

## Future Improvements

- Configurable depth limits via schema extensions
- Alternative hash algorithms for specific use cases
- Optimization for arrays with many identical nested structures
