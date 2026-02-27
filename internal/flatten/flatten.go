package flatten

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Options configures the flatten behavior.
type Options struct {
	Delimiter      string // Key delimiter (default "_")
	ArrayMode      string // "join", "index", or "json"
	ArrayJoinDelim string // Delimiter for array join mode (default ",")
	Upper          bool   // Uppercase key names
	NoFlatten      bool   // Disable JSON flattening
}

// DefaultOptions returns the default flatten options.
func DefaultOptions() Options {
	return Options{
		Delimiter:      "_",
		ArrayMode:      "join",
		ArrayJoinDelim: ",",
		Upper:          true,
		NoFlatten:      false,
	}
}

// Flatten takes a prefix and raw value string and returns a flattened key-value map.
// If rawValue is not valid JSON, it is treated as a raw string.
func Flatten(prefix, rawValue string, opts Options) (map[string]string, error) {
	result := make(map[string]string)

	if opts.NoFlatten {
		setKey(result, prefix, rawValue, opts)
		return result, nil
	}

	var v interface{}
	if err := json.Unmarshal([]byte(rawValue), &v); err != nil {
		// Not valid JSON: treat as raw string
		setKey(result, prefix, rawValue, opts)
		return result, nil
	}

	flattenAny(prefix, v, opts, result)
	return result, nil
}

// ApplyCasing applies hyphen-to-underscore replacement and upper/lower casing.
func ApplyCasing(key string, opts Options) string {
	k := strings.ReplaceAll(key, "-", "_")
	if opts.Upper {
		return strings.ToUpper(k)
	}
	return strings.ToLower(k)
}

// flattenAny dispatches to the appropriate handler based on value type.
func flattenAny(key string, v interface{}, opts Options, result map[string]string) {
	switch val := v.(type) {
	case map[string]interface{}:
		flattenObject(key, val, opts, result)
	case []interface{}:
		flattenArray(key, val, opts, result)
	case string:
		// If the string is valid JSON (e.g. a nested array/object encoded as a string),
		// try to parse and recursively flatten it.
		var nested interface{}
		if json.Unmarshal([]byte(val), &nested) == nil {
			// Only recurse if it parsed into a non-string type (object, array, etc.)
			if _, isStr := nested.(string); !isStr {
				flattenAny(key, nested, opts, result)
				return
			}
		}
		setKey(result, key, val, opts)
	case float64:
		setKey(result, key, formatNumber(val), opts)
	case bool:
		setKey(result, key, strconv.FormatBool(val), opts)
	case nil:
		setKey(result, key, "", opts)
	}
}

// flattenObject handles JSON object values.
func flattenObject(key string, obj map[string]interface{}, opts Options, result map[string]string) {
	for k, v := range obj {
		childKey := joinKey(key, k, opts.Delimiter)
		flattenAny(childKey, v, opts, result)
	}
}

// flattenArray handles JSON array values.
func flattenArray(key string, arr []interface{}, opts Options, result map[string]string) {
	if len(arr) == 0 {
		return
	}

	switch opts.ArrayMode {
	case "json":
		raw, err := json.Marshal(arr)
		if err != nil {
			// Fallback to index mode on marshal error (should not happen)
			flattenArrayByIndex(key, arr, opts, result)
			return
		}
		setKey(result, key, string(raw), opts)
	case "join":
		strs, ok := tryJoinStrings(arr)
		if ok {
			setKey(result, key, strings.Join(strs, opts.ArrayJoinDelim), opts)
		} else {
			// Fallback to index mode
			flattenArrayByIndex(key, arr, opts, result)
		}
	case "index":
		flattenArrayByIndex(key, arr, opts, result)
	default:
		flattenArrayByIndex(key, arr, opts, result)
	}
}

// flattenArrayByIndex expands each array element with an index suffix.
func flattenArrayByIndex(key string, arr []interface{}, opts Options, result map[string]string) {
	for i, elem := range arr {
		childKey := joinKey(key, strconv.Itoa(i), opts.Delimiter)
		flattenAny(childKey, elem, opts, result)
	}
}

// tryJoinStrings checks if all array elements are raw strings (not JSON-parseable).
// Returns the string slice and true if all elements qualify.
func tryJoinStrings(arr []interface{}) ([]string, bool) {
	strs := make([]string, 0, len(arr))
	for _, elem := range arr {
		s, ok := elem.(string)
		if !ok {
			return nil, false
		}
		// If the string is valid JSON, it should not be joined (needs further flattening)
		if json.Valid([]byte(s)) {
			return nil, false
		}
		strs = append(strs, s)
	}
	return strs, true
}

// joinKey joins prefix and suffix with the delimiter.
// If prefix is empty, returns suffix only.
func joinKey(prefix, suffix, delimiter string) string {
	if prefix == "" {
		return suffix
	}
	return prefix + delimiter + suffix
}

// setKey applies casing and stores the key-value pair in the result map.
func setKey(result map[string]string, key, value string, opts Options) {
	normalizedKey := ApplyCasing(key, opts)
	result[normalizedKey] = value
}

// formatNumber converts a float64 to a string, preferring integer format when possible.
func formatNumber(f float64) string {
	if f == float64(int64(f)) {
		return fmt.Sprintf("%d", int64(f))
	}
	return fmt.Sprintf("%g", f)
}
