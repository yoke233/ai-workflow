package sqlite

import "encoding/json"

// marshalJSON returns nil for nil/empty values, otherwise JSON bytes as string.
func marshalJSON(v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	s := string(b)
	if s == "null" || s == "{}" || s == "[]" {
		return nil, nil
	}
	return s, nil
}
