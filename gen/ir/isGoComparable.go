package ir

// IsGoComparable reports whether t can be compared with the Go == operator.
// Conservative: returns true only for primitive comparable kinds and enums.
// Byte slices and jx.Raw are not comparable.
func (t *Type) IsGoComparable() bool {
	if t == nil {
		return false
	}
	switch t.Kind {
	case KindPrimitive:
		goType := t.Go()
		if goType == "[]byte" || goType == "jx.Raw" {
			return false
		}
		return true
	case KindEnum:
		return true
	}
	return false
}
