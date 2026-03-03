package cmd

import (
	"encoding/json"
	"fmt"
	"io"
)

// printJSON marshals v to indented JSON and writes it to w followed by a newline.
func printJSON(w io.Writer, v any) error {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}
	_, err = fmt.Fprintln(w, string(out))
	return err
}
