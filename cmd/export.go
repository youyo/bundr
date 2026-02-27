package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/youyo/bundr/internal/backend"
	"github.com/youyo/bundr/internal/flatten"
	"github.com/youyo/bundr/internal/tags"
)

// ExportCmd represents the "export" subcommand.
type ExportCmd struct {
	From           string `required:"" help:"Source prefix (e.g. ps:/app/prod/)"`
	Format         string `default:"shell" enum:"shell,dotenv,direnv" help:"Output format"`
	NoFlatten      bool   `name:"no-flatten" help:"Disable JSON flattening"`
	ArrayMode      string `default:"join" enum:"join,index,json" help:"Array handling mode"`
	ArrayJoinDelim string `default:"," help:"Delimiter for array join mode"`
	FlattenDelim   string `default:"_" help:"Delimiter for flattened keys"`
	Upper          bool   `default:"true" negatable:"" help:"Uppercase key names"`

	out io.Writer // for testing; nil means os.Stdout
}

// Run executes the export command.
func (c *ExportCmd) Run(appCtx *Context) error {
	if c.out == nil {
		c.out = os.Stdout
	}

	ref, err := backend.ParseRef(c.From)
	if err != nil {
		return fmt.Errorf("export command failed: invalid ref: %w", err)
	}

	if ref.Type == backend.BackendTypeSM {
		return fmt.Errorf("export command failed: sm: backend is not supported (use ps: or psa:)")
	}

	b, err := appCtx.BackendFactory(ref.Type)
	if err != nil {
		return fmt.Errorf("export command failed: create backend: %w", err)
	}

	entries, err := b.GetByPrefix(context.Background(), ref.Path, backend.GetByPrefixOptions{Recursive: true})
	if err != nil {
		return fmt.Errorf("export command failed: %w", err)
	}

	flatOpts := flatten.Options{
		Delimiter:      c.FlattenDelim,
		ArrayMode:      c.ArrayMode,
		ArrayJoinDelim: c.ArrayJoinDelim,
		Upper:          c.Upper,
		NoFlatten:      c.NoFlatten,
	}

	vars := make(map[string]string)

	for _, entry := range entries {
		keyPrefix := pathToKey(entry.Path, ref.Path, c.FlattenDelim)

		if entry.StoreMode == tags.StoreModeJSON && !c.NoFlatten {
			kvs, err := flatten.Flatten(keyPrefix, entry.Value, flatOpts)
			if err != nil {
				return fmt.Errorf("export command failed: flatten %s: %w", entry.Path, err)
			}
			for k, v := range kvs {
				vars[k] = v
			}
		} else {
			normalizedKey := flatten.ApplyCasing(keyPrefix, flatOpts)
			vars[normalizedKey] = entry.Value
		}
	}

	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	formatter := newFormatter(c.Format)
	for _, k := range keys {
		fmt.Fprintln(c.out, formatter.line(k, vars[k]))
	}

	return nil
}

// pathToKey converts an SSM path to a key name by trimming the from prefix.
func pathToKey(path, fromPath, delim string) string {
	trimmed := strings.TrimPrefix(path, strings.TrimRight(fromPath, "/")+"/")
	return strings.ReplaceAll(trimmed, "/", delim)
}

// lineFormatter defines the output format.
type lineFormatter struct {
	prefix string
	quote  func(string) string
}

func newFormatter(format string) *lineFormatter {
	switch format {
	case "dotenv":
		return &lineFormatter{prefix: "", quote: dotenvQuote}
	case "shell", "direnv":
		return &lineFormatter{prefix: "export ", quote: shellQuote}
	default:
		return &lineFormatter{prefix: "export ", quote: shellQuote}
	}
}

func (f *lineFormatter) line(key, value string) string {
	return f.prefix + key + "=" + f.quote(value)
}

// shellQuote wraps a value in single quotes, escaping internal single quotes.
func shellQuote(s string) string {
	escaped := strings.ReplaceAll(s, "'", "'\"'\"'")
	return "'" + escaped + "'"
}

// dotenvQuote returns the value as-is if safe, or double-quoted with escaping.
func dotenvQuote(s string) string {
	if !needsDotenvQuote(s) {
		return s
	}
	escaped := strings.ReplaceAll(s, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}

// needsDotenvQuote checks if a value contains special characters requiring quoting.
func needsDotenvQuote(s string) bool {
	return strings.ContainsAny(s, " \t\n\"'\\#")
}
