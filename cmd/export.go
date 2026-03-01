package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/youyo/bundr/internal/backend"
	"github.com/youyo/bundr/internal/flatten"
	"github.com/youyo/bundr/internal/tags"
)

// VarsBuildOptions holds options for buildVars.
type VarsBuildOptions struct {
	From           string
	FlattenDelim   string
	ArrayMode      string
	ArrayJoinDelim string
	Upper          bool
	NoFlatten      bool
}

// buildVars fetches parameters from the given prefix and returns a key→value map.
// Used by both ExportCmd and RunCmd.
func buildVars(ctx context.Context, appCtx *Context, opts VarsBuildOptions) (map[string]string, error) {
	ref, err := backend.ParseRef(opts.From)
	if err != nil {
		return nil, fmt.Errorf("invalid ref: %w", err)
	}

	if ref.Type == backend.BackendTypeSM {
		return nil, fmt.Errorf("sm: backend is not supported (use ps: or psa:)")
	}

	b, err := appCtx.BackendFactory(ref.Type)
	if err != nil {
		return nil, fmt.Errorf("create backend: %w", err)
	}

	entries, err := b.GetByPrefix(ctx, ref.Path, backend.GetByPrefixOptions{Recursive: true})
	if err != nil {
		return nil, err
	}

	flatOpts := flatten.Options{
		Delimiter:      opts.FlattenDelim,
		ArrayMode:      opts.ArrayMode,
		ArrayJoinDelim: opts.ArrayJoinDelim,
		Upper:          opts.Upper,
		NoFlatten:      opts.NoFlatten,
	}

	vars := make(map[string]string)

	// Leaf parameter fallback: if GetByPrefix returns nothing and
	// the path does not end with "/", try Get for a single key.
	if len(entries) == 0 && !strings.HasSuffix(ref.Path, "/") {
		val, err := b.Get(ctx, opts.From, backend.GetOptions{})
		if err != nil {
			return nil, err
		}
		keyName := path.Base(ref.Path)
		normalizedKey := flatten.ApplyCasing(keyName, flatOpts)
		normalizedKey = strings.ReplaceAll(normalizedKey, ".", opts.FlattenDelim)
		vars[normalizedKey] = val
		return vars, nil
	}

	for _, entry := range entries {
		keyPrefix := pathToKey(entry.Path, ref.Path, opts.FlattenDelim)

		if entry.StoreMode == tags.StoreModeJSON && !opts.NoFlatten {
			kvs, err := flatten.Flatten(keyPrefix, entry.Value, flatOpts)
			if err != nil {
				return nil, fmt.Errorf("flatten %s: %w", entry.Path, err)
			}
			for k, v := range kvs {
				k = strings.ReplaceAll(k, ".", opts.FlattenDelim)
				vars[k] = v
			}
		} else {
			normalizedKey := flatten.ApplyCasing(keyPrefix, flatOpts)
			normalizedKey = strings.ReplaceAll(normalizedKey, ".", opts.FlattenDelim)
			vars[normalizedKey] = entry.Value
		}
	}

	return vars, nil
}

// ExportCmd represents the "export" subcommand.
type ExportCmd struct {
	From           string `arg:"" required:"" predictor:"prefix" help:"Source prefix (e.g. ps:/app/prod/)"`
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

	vars, err := buildVars(context.Background(), appCtx, VarsBuildOptions{
		From:           c.From,
		FlattenDelim:   c.FlattenDelim,
		ArrayMode:      c.ArrayMode,
		ArrayJoinDelim: c.ArrayJoinDelim,
		Upper:          c.Upper,
		NoFlatten:      c.NoFlatten,
	})
	if err != nil {
		return fmt.Errorf("export command failed: %w", err)
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
