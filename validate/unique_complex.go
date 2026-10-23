package validate

import (
	"bytes"
	"math/big"
	"reflect"

	"github.com/go-faster/errors"
	"github.com/go-faster/jx"
)

// UniqueItemsDeep ensures given array has no duplicates, comparing elements
// with EqualDeep.
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
			if EqualDeep(arr[i], arr[j]) {
				return &DuplicateItemsError{Indices: []int{i, j}}
			}
		}
	}
	return nil
}

// EqualDeep reports whether two values of the same type are deeply equal,
// comparing raw JSON payloads semantically instead of byte by byte.
//
// It matches reflect.DeepEqual, including the distinction between nil and
// empty slices and maps, except that jx.Raw values (and named types whose
// underlying type is a raw JSON byte slice) compare through EqualRawJSON,
// and types with an Equal method of the form Equal(T) bool delegate to it,
// so generated struct items, time.Time and decimal values compare by value.
func EqualDeep[T any](a, b T) bool {
	return equalDeepValue(reflect.ValueOf(a), reflect.ValueOf(b))
}

var (
	rawJSONReflectType   = reflect.TypeOf(jx.Raw(nil))
	byteSliceReflectType = reflect.TypeOf([]byte(nil))
	boolReflectType      = reflect.TypeOf(false)
)

// isRawJSONReflectType reports whether a reflect type holds raw JSON bytes:
// jx.Raw itself, or a defined byte-slice type such as a generated alias of a
// free-form schema. Plain []byte stays bytewise, because it carries decoded
// binary data, not JSON.
func isRawJSONReflectType(t reflect.Type) bool {
	if t == rawJSONReflectType {
		return true
	}
	return t != byteSliceReflectType &&
		t.Kind() == reflect.Slice &&
		t.Elem().Kind() == reflect.Uint8 &&
		t.ConvertibleTo(rawJSONReflectType)
}

func equalDeepValue(a, b reflect.Value) bool {
	if !a.IsValid() || !b.IsValid() {
		return a.IsValid() == b.IsValid()
	}
	t := a.Type()
	if t != b.Type() {
		return false
	}
	if isRawJSONReflectType(t) {
		return EqualRawJSON(jx.Raw(a.Bytes()), jx.Raw(b.Bytes()))
	}
	if method, ok := t.MethodByName("Equal"); ok &&
		method.Type.NumIn() == 2 && method.Type.In(1) == t &&
		method.Type.NumOut() == 1 && method.Type.Out(0) == boolReflectType {
		return method.Func.Call([]reflect.Value{a, b})[0].Bool()
	}
	switch t.Kind() {
	case reflect.Pointer:
		if a.IsNil() || b.IsNil() {
			return a.IsNil() == b.IsNil()
		}
		return equalDeepValue(a.Elem(), b.Elem())
	case reflect.Interface:
		if a.IsNil() || b.IsNil() {
			return a.IsNil() == b.IsNil()
		}
		return equalDeepValue(a.Elem(), b.Elem())
	case reflect.Slice:
		if a.IsNil() != b.IsNil() || a.Len() != b.Len() {
			return false
		}
		for i := 0; i < a.Len(); i++ {
			if !equalDeepValue(a.Index(i), b.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Array:
		for i := 0; i < a.Len(); i++ {
			if !equalDeepValue(a.Index(i), b.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Map:
		if a.IsNil() != b.IsNil() || a.Len() != b.Len() {
			return false
		}
		iter := a.MapRange()
		for iter.Next() {
			bval := b.MapIndex(iter.Key())
			if !bval.IsValid() || !equalDeepValue(iter.Value(), bval) {
				return false
			}
		}
		return true
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			// Unexported fields cannot be read through reflection
			// without unsafe, so compare the whole struct bytewise
			if t.Field(i).PkgPath != "" {
				return reflect.DeepEqual(a.Interface(), b.Interface())
			}
		}
		for i := 0; i < t.NumField(); i++ {
			if !equalDeepValue(a.Field(i), b.Field(i)) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(a.Interface(), b.Interface())
	}
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
// exactly for any magnitude, without losing precision to a float64 round-trip
// and without parser range limits on the exponent. Malformed input falls back
// to byte comparison.
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
	lneg, ldigits, lexp, err := normalizeNumberLiteral(lval)
	if err != nil {
		return false, errors.Wrap(err, "left")
	}
	rneg, rdigits, rexp, err := normalizeNumberLiteral(rval)
	if err != nil {
		return false, errors.Wrap(err, "right")
	}
	if ldigits == "" || rdigits == "" {
		// Zero in any notation equals zero, regardless of sign or exponent
		return ldigits == rdigits, nil
	}
	return lneg == rneg && ldigits == rdigits && lexp.Cmp(rexp) == 0, nil
}

// normalizeNumberLiteral decomposes a JSON number literal into a sign, its
// significant digits and a decimal exponent, so that the value equals
// digits * 10^exponent with the decimal point after the last digit. The
// exponent is a big.Int, so arbitrarily large exponents normalize without
// overflow, unlike parsers that expand the value or use machine integers.
// A zero value yields empty digits.
func normalizeNumberLiteral(literal []byte) (negative bool, digits string, exponent *big.Int, _ error) {
	rest := literal
	if len(rest) > 0 && rest[0] == '-' {
		negative = true
		rest = rest[1:]
	}
	takeDigits := func() string {
		i := 0
		for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
			i++
		}
		taken := string(rest[:i])
		rest = rest[i:]
		return taken
	}
	integer := takeDigits()
	if integer == "" {
		return false, "", nil, errors.Errorf("invalid number literal %q", literal)
	}
	fraction := ""
	if len(rest) > 0 && rest[0] == '.' {
		rest = rest[1:]
		fraction = takeDigits()
		if fraction == "" {
			return false, "", nil, errors.Errorf("invalid number literal %q", literal)
		}
	}
	exponent = new(big.Int)
	if len(rest) > 0 && (rest[0] == 'e' || rest[0] == 'E') {
		rest = rest[1:]
		if _, ok := exponent.SetString(string(rest), 10); !ok {
			return false, "", nil, errors.Errorf("invalid number literal %q", literal)
		}
		rest = nil
	}
	if len(rest) > 0 {
		return false, "", nil, errors.Errorf("invalid number literal %q", literal)
	}

	mantissa := integer + fraction
	exponent.Sub(exponent, big.NewInt(int64(len(fraction))))
	start := 0
	for start < len(mantissa) && mantissa[start] == '0' {
		start++
	}
	end := len(mantissa)
	for end > start && mantissa[end-1] == '0' {
		end--
	}
	exponent.Add(exponent, big.NewInt(int64(len(mantissa)-end)))
	return negative, mantissa[start:end], exponent, nil
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
