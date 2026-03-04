package dotenv

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Entry represents a single key-value pair in a .env file.
type Entry struct {
	Key   string
	Value string
}

// Parse reads a .env format from r and returns the parsed entries.
// It supports comment lines (# prefix), blank lines, and three value formats:
// KEY='value', KEY="value", KEY=value.
// Values containing '=' are handled correctly (KEY=a=b → value is "a=b").
func Parse(r io.Reader) ([]Entry, error) {
	var entries []Entry
	scanner := bufio.NewScanner(r)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			return nil, fmt.Errorf("line %d: missing '='", lineNum)
		}

		key := strings.TrimSpace(line[:idx])
		if key == "" {
			return nil, fmt.Errorf("line %d: empty key", lineNum)
		}

		val := line[idx+1:]
		val = unquote(val)

		entries = append(entries, Entry{Key: key, Value: val})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	return entries, nil
}

// Write outputs entries in KEY=VALUE format to w.
func Write(w io.Writer, entries []Entry) error {
	for _, e := range entries {
		if _, err := fmt.Fprintf(w, "%s=%s\n", e.Key, e.Value); err != nil {
			return err
		}
	}
	return nil
}

// WriteExport outputs entries in "export KEY=VALUE" format to w.
// Suitable for use with eval: eval $(bundr sync -f ... -t -)
func WriteExport(w io.Writer, entries []Entry) error {
	for _, e := range entries {
		if _, err := fmt.Fprintf(w, "export %s=%s\n", e.Key, e.Value); err != nil {
			return err
		}
	}
	return nil
}

// unquote removes matching surrounding quotes (single or double) from s.
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') ||
			(s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
