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

// UniqueItemsDeep ensures given array has no duplicates, comparing elements
// with EqualDeep.
//
// It is used for uniqueItems arrays whose element type is not comparable
// with == and has no generated Equal method, such as nested arrays, maps
// and nullable object wrappers. When EqualDeep exhausts its recursion
// budget, the comparison is reported as a DepthLimitError instead of a
// silently wrong verdict.
func UniqueItemsDeep[S ~[]T, T any](arr S) (err error) {
	if len(arr) < 2 {
		return nil
	}
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(*DepthLimitError); ok {
				err = e
			} else {
				panic(r)
			}
		}
	}()
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
// empty slices and maps, except that jx.Raw values compare through
// EqualRawJSON, time.Time values compare through EqualTime, types with an
// EqualDepth(T, depth int) bool method delegate to it with the current
// depth, and types with an Equal method of the form Equal(T) bool delegate
// to that, so generated wrappers, generated raw JSON aliases and decimal
// values compare by value. Named byte-slice types without an Equal method
// stay bytewise, because they carry decoded binary data, not JSON.
//
// Comparison depth is bounded: beyond equalDeepMaxDepth levels, such as on
// cyclic values, EqualDeep panics with a DepthLimitError, which
// UniqueItemsDeep and the generated uniqueness validators recover into a
// returned error. Generated wrapper EqualDepth methods receive the current
// depth and forward it back through EqualDeepDepth, so one budget spans the
// whole comparison instead of restarting at every wrapper boundary.
func EqualDeep[T any](a, b T) bool {
	return equalDeepValue(reflect.ValueOf(a), reflect.ValueOf(b), 0)
}

// EqualDeepDepth continues a deep comparison at the given recursion depth.
// Generated EqualDepth methods call it with the depth they received, so one
// recursion budget spans the whole comparison and cyclic values cannot
// bypass the limit by resetting it at each wrapper boundary.
func EqualDeepDepth[T any](a, b T, depth int) bool {
	return equalDeepValue(reflect.ValueOf(a), reflect.ValueOf(b), depth)
}

// equalDeepMaxDepth bounds recursion in equalDeepValue, so cyclic values
// surface as an error instead of exhausting the stack.
const equalDeepMaxDepth = 1000

var (
	rawJSONReflectType = reflect.TypeOf(jx.Raw(nil))
	timeReflectType    = reflect.TypeOf(time.Time{})
	decimalReflectType = reflect.TypeOf(decimal.Decimal{})
	boolReflectType    = reflect.TypeOf(false)
	intReflectType     = reflect.TypeOf(0)
)

func equalDeepValue(a, b reflect.Value, depth int) bool {
	if !a.IsValid() || !b.IsValid() {
		return a.IsValid() == b.IsValid()
	}
	t := a.Type()
	if t != b.Type() {
		return false
	}
	if depth > equalDeepMaxDepth {
		panic(&DepthLimitError{
			MaxDepth: equalDeepMaxDepth,
			TypeName: t.String(),
		})
	}
	if t == rawJSONReflectType {
		return EqualRawJSON(jx.Raw(a.Bytes()), jx.Raw(b.Bytes()))
	}
	if t == timeReflectType {
		return EqualTime(a.Interface().(time.Time), b.Interface().(time.Time))
	}
	if t.Kind() == reflect.Struct && t.ConvertibleTo(decimalReflectType) {
		// decimal.Decimal and defined types over it, such as a referenced
		// decimal schema, compare by numeric value. Defined types have no
		// promoted Equal method, so without this case reflection would fall
		// back to comparing the stored coefficient and exponent, making
		// numerically equal decimals with different scales distinct
		left := a.Convert(decimalReflectType).Interface().(decimal.Decimal)
		right := b.Convert(decimalReflectType).Interface().(decimal.Decimal)
		return left.Equal(right)
	}
	// Generated wrapper methods continue the shared recursion budget, so
	// cyclic values cannot bypass the depth limit by restarting it at every
	// wrapper boundary
	if method, ok := t.MethodByName("EqualDepth"); ok &&
		method.Type.NumIn() == 3 && method.Type.In(1) == t &&
		method.Type.In(2) == intReflectType &&
		method.Type.NumOut() == 1 && method.Type.Out(0) == boolReflectType {
		return method.Func.Call([]reflect.Value{a, b, reflect.ValueOf(depth + 1)})[0].Bool()
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
		return equalDeepValue(a.Elem(), b.Elem(), depth+1)
	case reflect.Interface:
		if a.IsNil() || b.IsNil() {
			return a.IsNil() == b.IsNil()
		}
		return equalDeepValue(a.Elem(), b.Elem(), depth+1)
	case reflect.Slice:
		if a.IsNil() != b.IsNil() || a.Len() != b.Len() {
			return false
		}
		for i := 0; i < a.Len(); i++ {
			if !equalDeepValue(a.Index(i), b.Index(i), depth+1) {
				return false
			}
		}
		return true
	case reflect.Array:
		for i := 0; i < a.Len(); i++ {
			if !equalDeepValue(a.Index(i), b.Index(i), depth+1) {
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
			if !bval.IsValid() || !equalDeepValue(iter.Value(), bval, depth+1) {
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
			if !equalDeepValue(a.Field(i), b.Field(i), depth+1) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(a.Interface(), b.Interface())
	}
}

// EqualTime reports whether two timestamps denote the same instant with the
// same UTC offset. Comparing time.Time with == also compares the location
// pointer, so identical decoded literals with a numeric zone offset would
// not be equal, while Equal alone ignores the offset, so distinct
// representations of one instant would collapse.
func EqualTime(a, b time.Time) bool {
	if !a.Equal(b) {
		return false
	}
	_, aoffset := a.Zone()
	_, boffset := b.Zone()
	return aoffset == boffset
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
	lmembers, lcount, err := objectMembers(left)
	if err != nil {
		return false, errors.Wrap(err, "left")
	}
	rmembers, rcount, err := objectMembers(right)
	if err != nil {
		return false, errors.Wrap(err, "right")
	}
	// Repeated keys collapse to their last value on both sides, so the
	// verdict does not depend on operand order
	if lcount != rcount || len(lmembers) != len(rmembers) {
		return false, nil
	}
	for key, lval := range lmembers {
		rval, ok := rmembers[key]
		if !ok {
			return false, nil
		}
		equal, err := rawJSONEqual(lval, rval)
		if err != nil {
			return false, err
		}
		if !equal {
			return false, nil
		}
	}
	return true, nil
}

// objectMembers collects the members of a JSON object, keeping the last
// value of a repeated key, together with the total member count.
func objectMembers(d *jx.Decoder) (map[string]jx.Raw, int, error) {
	members := map[string]jx.Raw{}
	count := 0
	if err := d.ObjBytes(func(d *jx.Decoder, key []byte) error {
		raw, err := d.Raw()
		if err != nil {
			return errors.Wrapf(err, "%q", key)
		}
		members[string(key)] = raw
		count++
		return nil
	}); err != nil {
		return nil, 0, err
	}
	return members, count, nil
}
