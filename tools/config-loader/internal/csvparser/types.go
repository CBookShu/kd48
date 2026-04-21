package csvparser

import (
	"strconv"
	"strings"

	"github.com/CBookShu/kd48/tools/config-loader/internal/errors"
)

func ParseValue(raw, typ string) (Value, error) {
	v := Value{
		Raw:     raw,
		Type:    typ,
		IsEmpty: strings.TrimSpace(raw) == "",
	}

	switch typ {
	case "int32":
		return parseInt32(v, raw)
	case "int64":
		return parseInt64(v, raw)
	case "string":
		v.Parsed = raw
		return v, nil
	case "time":
		return parseTime(v, raw)
	case "int32[]":
		return parseInt32Array(v, raw)
	case "int64[]":
		return parseInt64Array(v, raw)
	case "string[]":
		return parseStringArray(v, raw)
	default:
		if strings.Contains(typ, "=") {
			return parseMap(v, raw, typ)
		}
		v.Parsed = raw
		return v, nil
	}
}

func parseInt32(v Value, raw string) (Value, error) {
	if v.IsEmpty {
		v.Parsed = int32(0)
		return v, nil
	}
	n, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return v, errors.New(errors.ErrInvalidValue, "invalid int32 value", -1, -1, raw)
	}
	v.Parsed = int32(n)
	return v, nil
}

func parseInt64(v Value, raw string) (Value, error) {
	if v.IsEmpty {
		v.Parsed = int64(0)
		return v, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return v, errors.New(errors.ErrInvalidValue, "invalid int64 value", -1, -1, raw)
	}
	v.Parsed = n
	return v, nil
}

func parseTime(v Value, raw string) (Value, error) {
	if v.IsEmpty {
		return v, errors.New(errors.ErrTimeEmpty, "time field cannot be empty", -1, -1, raw)
	}
	v.Parsed = raw
	return v, nil
}

func parseInt32Array(v Value, raw string) (Value, error) {
	if v.IsEmpty {
		v.Parsed = []int32{}
		return v, nil
	}
	parts := strings.Split(raw, "|")
	arr := make([]int32, len(parts))
	for i, p := range parts {
		if strings.TrimSpace(p) == "" {
			arr[i] = 0
			continue
		}
		n, err := strconv.ParseInt(p, 10, 32)
		if err != nil {
			return v, errors.New(errors.ErrInvalidValue, "invalid int32 in array", -1, -1, p)
		}
		arr[i] = int32(n)
	}
	v.Parsed = arr
	return v, nil
}

func parseInt64Array(v Value, raw string) (Value, error) {
	if v.IsEmpty {
		v.Parsed = []int64{}
		return v, nil
	}
	parts := strings.Split(raw, "|")
	arr := make([]int64, len(parts))
	for i, p := range parts {
		if strings.TrimSpace(p) == "" {
			arr[i] = 0
			continue
		}
		n, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			return v, errors.New(errors.ErrInvalidValue, "invalid int64 in array", -1, -1, p)
		}
		arr[i] = n
	}
	v.Parsed = arr
	return v, nil
}

func parseStringArray(v Value, raw string) (Value, error) {
	if v.IsEmpty {
		v.Parsed = []string{}
		return v, nil
	}
	parts := strings.Split(raw, "|")
	arr := make([]string, len(parts))
	for i, p := range parts {
		arr[i] = unquote(strings.TrimSpace(p))
	}
	v.Parsed = arr
	return v, nil
}

func parseMap(v Value, raw, typ string) (Value, error) {
	if v.IsEmpty {
		v.Parsed = map[string]interface{}{}
		return v, nil
	}

	parts := strings.Split(typ, "=")
	keyType := strings.TrimSpace(parts[0])

	entries := strings.Split(raw, "|")
	result := make(map[string]interface{})

	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		kv := splitKeyValue(entry)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])

		key = unquote(key)
		value = unquote(value)

		if keyType == "int32" || keyType == "int64" {
			n, _ := strconv.ParseInt(key, 10, 64)
			result[strconv.FormatInt(n, 10)] = value
		} else {
			result[key] = value
		}
	}
	v.Parsed = result
	return v, nil
}

func splitKeyValue(s string) []string {
	inQuote := false
	for i, c := range s {
		if c == '\'' || c == '"' {
			inQuote = !inQuote
		}
		if c == '=' && !inQuote {
			return []string{s[:i], s[i+1:]}
		}
	}
	return strings.SplitN(s, "=", 2)
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '\'' && s[len(s)-1] == '\'' || s[0] == '"' && s[len(s)-1] == '"') {
		return s[1 : len(s)-1]
	}
	return s
}
