package validate

import (
	"bytes"
	"math/big"
	"reflect"
	"time"

	"github.com/go-faster/errors"
	"github.com/go-faster/jx"
	"github.com/shopspring/decimal"
)

// jsonRawType is the reflect type of jx.Raw, which stores free-form JSON
// values such as untyped properties and additionalProperties without a
// declared type. Comparing those byte slices directly would make equality
// depend on member order, whitespace and escaping, so deepEqualAny compares
// them as JSON values instead.
var (
	jsonRawType = reflect.TypeOf(jx.Raw(nil))
	timeType    = reflect.TypeOf(time.Time{})
	decimalType = reflect.TypeOf(decimal.Decimal{})
)

// isJSONRawType reports whether a reflect type holds raw JSON bytes. Only
// jx.Raw itself qualifies: a defined byte-slice type can be a generated
// alias of a referenced binary schema, and reflection cannot tell it apart
// from an alias of a free-form schema, so treating its bytes as JSON would
// conflate distinct binary payloads that happen to parse as JSON. Defined
// byte slices carrying a generated Encode method are compared through
// their JSON encoding in deepEqualValue instead; the rest stay bytewise.
func isJSONRawType(t reflect.Type) bool {
	return t == jsonRawType
}

// errExactNumber reports a raw JSON number the comparison cannot represent
// exactly. Returning an error is preferable to silently treating equivalent
// numbers as different, or different numbers as equal.
var errExactNumber = errors.New("uniqueItems: number too large to compare exactly")

// deepEqualAny compares two values like reflect.DeepEqual, except that
// jx.Raw payloads are compared as JSON values.
func deepEqualAny(a, b any) (bool, error) {
	return deepEqualValue(reflect.ValueOf(a), reflect.ValueOf(b))
}

func deepEqualValue(a, b reflect.Value) (bool, error) {
	if !a.IsValid() || !b.IsValid() {
		return a.IsValid() == b.IsValid(), nil
	}
	if a.Type() != b.Type() {
		return false, nil
	}
	if isJSONRawType(a.Type()) {
		return equalJSONRaw(a.Bytes(), b.Bytes())
	}
	if a.Kind() == reflect.Slice && a.Type().Elem().Kind() == reflect.Uint8 {
		if left, ok := a.Interface().(interface{ Encode(*jx.Encoder) }); ok {
			// Generated named types over byte slices carry their own JSON
			// encoding: an alias of a referenced free-form schema encodes
			// its raw JSON verbatim while an alias of a referenced binary
			// schema encodes a base64 string, so comparing the encodings as
			// JSON values preserves both semantics, which reflection on the
			// byte slice alone cannot tell apart
			if a.IsNil() || b.IsNil() {
				return a.IsNil() == b.IsNil(), nil
			}
			right := b.Interface().(interface{ Encode(*jx.Encoder) })
			var leftEncoded, rightEncoded jx.Encoder
			left.Encode(&leftEncoded)
			right.Encode(&rightEncoded)
			return equalJSONRaw(leftEncoded.Bytes(), rightEncoded.Bytes())
		}
	}
	if a.Type() == timeType {
		// == and reflect.DeepEqual also compare the location pointer, so
		// identical decoded timestamps with a numeric zone offset would not
		// match, while Equal alone ignores the offset, so distinct
		// representations of one instant would collapse
		left, right := a.Interface().(time.Time), b.Interface().(time.Time)
		if !left.Equal(right) {
			return false, nil
		}
		_, leftOffset := left.Zone()
		_, rightOffset := right.Zone()
		return leftOffset == rightOffset, nil
	}
	if a.Type().Kind() == reflect.Struct && a.Type().ConvertibleTo(decimalType) {
		// decimal.Decimal and defined types over it compare by numeric
		// value: the struct fields hold a coefficient and exponent, so a
		// field-by-field comparison would make numerically equal decimals
		// with different scales distinct
		left := a.Convert(decimalType).Interface().(decimal.Decimal)
		right := b.Convert(decimalType).Interface().(decimal.Decimal)
		return left.Equal(right), nil
	}
	switch a.Kind() {
	case reflect.Pointer, reflect.Interface:
		if a.IsNil() || b.IsNil() {
			return a.IsNil() == b.IsNil(), nil
		}
		return deepEqualValue(a.Elem(), b.Elem())
	case reflect.Slice:
		// nil encodes an absent or null array, which stays distinct from
		// a present empty array
		if a.IsNil() != b.IsNil() || a.Len() != b.Len() {
			return false, nil
		}
		for i := 0; i < a.Len(); i++ {
			equal, err := deepEqualValue(a.Index(i), b.Index(i))
			if err != nil || !equal {
				return false, err
			}
		}
		return true, nil
	case reflect.Map:
		if a.IsNil() != b.IsNil() || a.Len() != b.Len() {
			return false, nil
		}
		iter := a.MapRange()
		for iter.Next() {
			bval := b.MapIndex(iter.Key())
			if !bval.IsValid() {
				return false, nil
			}
			equal, err := deepEqualValue(iter.Value(), bval)
			if err != nil || !equal {
				return false, err
			}
		}
		return true, nil
	case reflect.Struct:
		for i := 0; i < a.NumField(); i++ {
			// Unexported fields are not readable without unsafe, so fall
			// back to reflect.DeepEqual for the whole struct
			if a.Type().Field(i).PkgPath != "" {
				return reflect.DeepEqual(a.Interface(), b.Interface()), nil
			}
		}
		for i := 0; i < a.NumField(); i++ {
			equal, err := deepEqualValue(a.Field(i), b.Field(i))
			if err != nil || !equal {
				return false, err
			}
		}
		return true, nil
	default:
		return reflect.DeepEqual(a.Interface(), b.Interface()), nil
	}
}

// equalJSONRaw compares two raw JSON payloads as values: object member
// order, insignificant whitespace and string escaping do not affect the
// result, and numbers compare exactly through big.Rat without a float64
// round-trip. Numbers the parser cannot represent produce errExactNumber
// instead of a silently wrong answer. Payloads that are not valid JSON,
// including absent values stored as empty slices, compare bytewise.
func equalJSONRaw(a, b []byte) (bool, error) {
	if len(a) == 0 || len(b) == 0 {
		return bytes.Equal(a, b), nil
	}
	left, right := jx.DecodeBytes(a), jx.DecodeBytes(b)
	equal, err := equalJSONDecoded(left, right)
	if err != nil {
		if errors.Is(err, errExactNumber) {
			return false, err
		}
		return bytes.Equal(a, b), nil
	}
	return equal, nil
}

func equalJSONDecoded(left, right *jx.Decoder) (bool, error) {
	ltype, rtype := left.Next(), right.Next()
	if ltype == jx.Invalid {
		return false, errors.New("invalid JSON")
	}
	if rtype == jx.Invalid {
		return false, errors.New("invalid JSON")
	}
	if ltype != rtype {
		return false, nil
	}
	switch ltype {
	case jx.Null:
		if err := left.Null(); err != nil {
			return false, err
		}
		if err := right.Null(); err != nil {
			return false, err
		}
		return true, nil
	case jx.Bool:
		lval, err := left.Bool()
		if err != nil {
			return false, err
		}
		rval, err := right.Bool()
		if err != nil {
			return false, err
		}
		return lval == rval, nil
	case jx.String:
		lval, err := left.StrBytes()
		if err != nil {
			return false, err
		}
		rval, err := right.StrBytes()
		if err != nil {
			return false, err
		}
		return bytes.Equal(lval, rval), nil
	case jx.Number:
		lval, err := left.Num()
		if err != nil {
			return false, err
		}
		rval, err := right.Num()
		if err != nil {
			return false, err
		}
		if bytes.Equal(lval, rval) {
			return true, nil
		}
		lnum, ok := new(big.Rat).SetString(string(lval))
		if !ok {
			return false, errExactNumber
		}
		rnum, ok := new(big.Rat).SetString(string(rval))
		if !ok {
			return false, errExactNumber
		}
		return lnum.Cmp(rnum) == 0, nil
	case jx.Array:
		liter, err := left.ArrIter()
		if err != nil {
			return false, err
		}
		riter, err := right.ArrIter()
		if err != nil {
			return false, err
		}
		for liter.Next() {
			if !riter.Next() {
				return false, nil
			}
			equal, err := equalJSONDecoded(left, right)
			if err != nil || !equal {
				return false, err
			}
		}
		if err := liter.Err(); err != nil {
			return false, err
		}
		if err := riter.Err(); err != nil {
			return false, err
		}
		return !riter.Next(), nil
	case jx.Object:
		collect := func(d *jx.Decoder) (map[string]jx.Raw, int, error) {
			members := map[string]jx.Raw{}
			count := 0
			if err := d.ObjBytes(func(d *jx.Decoder, key []byte) error {
				raw, err := d.Raw()
				if err != nil {
					return err
				}
				members[string(key)] = raw
				count++
				return nil
			}); err != nil {
				return nil, 0, err
			}
			return members, count, nil
		}
		lmembers, lcount, err := collect(left)
		if err != nil {
			return false, err
		}
		rmembers, rcount, err := collect(right)
		if err != nil {
			return false, err
		}
		// Repeated keys collapse to their last value on both sides, so the
		// verdict does not depend on operand order
		if lcount != rcount || len(lmembers) != len(rmembers) {
			return false, nil
		}
		for key, lraw := range lmembers {
			rraw, ok := rmembers[key]
			if !ok {
				return false, nil
			}
			equal, err := equalJSONRaw(lraw, rraw)
			if err != nil || !equal {
				return false, err
			}
		}
		return true, nil
	default:
		return false, errors.New("unexpected JSON type")
	}
}
