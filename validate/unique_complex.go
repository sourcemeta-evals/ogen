package validate

import (
	"bytes"
	"math/big"
	"reflect"

	"github.com/go-faster/errors"
	"github.com/go-faster/jx"
)

// UniqueItemsDeep ensures given array has no duplicates, comparing elements
// with reflect.DeepEqual.
//
// It is used for uniqueItems arrays whose element type is not comparable
// with == and has no generated Equal method, such as nested arrays, maps
// and nullable object wrappers.
func UniqueItemsDeep[S ~[]T, T any](arr S) error {
	if len(arr) < 2 {
		return nil
	}
	for i := range arr {
		for j := i + 1; j < len(arr); j++ {
			if reflect.DeepEqual(arr[i], arr[j]) {
				return &DuplicateItemsError{Indices: []int{i, j}}
			}
		}
	}
	return nil
}

// UniqueItemsRawJSON ensures given array of raw JSON values has no
// semantically duplicate elements.
func UniqueItemsRawJSON[S ~[]T, T ~[]byte](arr S) error {
	if len(arr) < 2 {
		return nil
	}
	for i := range arr {
		for j := i + 1; j < len(arr); j++ {
			if EqualRawJSON(jx.Raw(arr[i]), jx.Raw(arr[j])) {
				return &DuplicateItemsError{Indices: []int{i, j}}
			}
		}
	}
	return nil
}

// EqualRawJSON reports whether two raw JSON values are semantically equal:
// object member order, insignificant whitespace, string escaping and
// equivalent numeric notation do not affect the result. Numbers are compared
// exactly, without losing precision to a float64 round-trip. Malformed input
// falls back to byte comparison.
func EqualRawJSON(a, b jx.Raw) bool {
	equal, err := rawJSONEqual(a, b)
	if err != nil {
		return bytes.Equal(a, b)
	}
	return equal
}

func rawJSONEqual(a, b jx.Raw) (bool, error) {
	left, right := jx.GetDecoder(), jx.GetDecoder()
	defer func() {
		jx.PutDecoder(left)
		jx.PutDecoder(right)
	}()
	left.ResetBytes(a)
	right.ResetBytes(b)
	return equalJSONValue(left, right)
}

func equalJSONValue(left, right *jx.Decoder) (bool, error) {
	ltyp, rtyp := left.Next(), right.Next()
	switch {
	case ltyp == jx.Invalid:
		return false, errors.Wrap(left.Validate(), "left")
	case rtyp == jx.Invalid:
		return false, errors.Wrap(right.Validate(), "right")
	case ltyp != rtyp:
		return false, nil
	}

	switch ltyp {
	case jx.Null:
		// Consume both nulls so iteration over an enclosing container
		// resumes at the next element.
		if err := left.Null(); err != nil {
			return false, errors.Wrap(err, "left")
		}
		if err := right.Null(); err != nil {
			return false, errors.Wrap(err, "right")
		}
		return true, nil
	case jx.Bool:
		lval, err := left.Bool()
		if err != nil {
			return false, errors.Wrap(err, "left")
		}
		rval, err := right.Bool()
		if err != nil {
			return false, errors.Wrap(err, "right")
		}
		return lval == rval, nil
	case jx.String:
		lval, err := left.StrBytes()
		if err != nil {
			return false, errors.Wrap(err, "left")
		}
		rval, err := right.StrBytes()
		if err != nil {
			return false, errors.Wrap(err, "right")
		}
		return bytes.Equal(lval, rval), nil
	case jx.Number:
		return equalJSONNumber(left, right)
	case jx.Array:
		return equalJSONArray(left, right)
	case jx.Object:
		return equalJSONObject(left, right)
	default:
		return false, errors.Errorf("unexpected type %v", ltyp)
	}
}

func equalJSONNumber(left, right *jx.Decoder) (bool, error) {
	lval, err := left.Num()
	if err != nil {
		return false, errors.Wrap(err, "left")
	}
	rval, err := right.Num()
	if err != nil {
		return false, errors.Wrap(err, "right")
	}
	switch {
	case lval.Zero() && rval.Zero():
		return true, nil
	case lval.Equal(rval):
		return true, nil
	case lval.IsInt() && rval.IsInt():
		// Integers without fraction or exponent notation that differ as
		// bytes also differ as numbers.
		return false, nil
	}
	lnum, rnum := new(big.Rat), new(big.Rat)
	if err := lnum.UnmarshalText(lval); err != nil {
		return false, errors.Wrap(err, "left")
	}
	if err := rnum.UnmarshalText(rval); err != nil {
		return false, errors.Wrap(err, "right")
	}
	return lnum.Cmp(rnum) == 0, nil
}

func equalJSONArray(left, right *jx.Decoder) (bool, error) {
	liter, err := left.ArrIter()
	if err != nil {
		return false, errors.Wrap(err, "left")
	}
	riter, err := right.ArrIter()
	if err != nil {
		return false, errors.Wrap(err, "right")
	}
	for liter.Next() {
		if !riter.Next() {
			return false, nil
		}
		equal, err := equalJSONValue(left, right)
		if err != nil {
			return false, err
		}
		if !equal {
			return false, nil
		}
	}
	if err := liter.Err(); err != nil {
		return false, errors.Wrap(err, "left")
	}
	if err := riter.Err(); err != nil {
		return false, errors.Wrap(err, "right")
	}
	return !riter.Next(), nil
}

func equalJSONObject(left, right *jx.Decoder) (bool, error) {
	lmembers := map[string]jx.Raw{}
	if err := left.ObjBytes(func(d *jx.Decoder, key []byte) error {
		raw, err := d.Raw()
		if err != nil {
			return errors.Wrapf(err, "%q", key)
		}
		lmembers[string(key)] = raw
		return nil
	}); err != nil {
		return false, errors.Wrap(err, "left")
	}

	matched := 0
	riter, err := right.ObjIter()
	if err != nil {
		return false, errors.Wrap(err, "right")
	}
	for riter.Next() {
		lval, ok := lmembers[string(riter.Key())]
		if !ok {
			return false, nil
		}
		matched++

		rval, err := right.Raw()
		if err != nil {
			return false, errors.Wrap(err, "right")
		}
		equal, err := rawJSONEqual(lval, rval)
		if err != nil {
			return false, err
		}
		if !equal {
			return false, nil
		}
	}
	if err := riter.Err(); err != nil {
		return false, errors.Wrap(err, "right")
	}
	return matched == len(lmembers), nil
}
