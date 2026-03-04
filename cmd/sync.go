package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/youyo/bundr/internal/backend"
	"github.com/youyo/bundr/internal/dotenv"
	"github.com/youyo/bundr/internal/tags"
)

// SyncCmd represents the "sync" subcommand.
type SyncCmd struct {
	From   string `required:"" short:"f" help:"Source: file path, -, ps:/path, ps:/prefix/, sm:id"`
	To     string `required:"" short:"t" help:"Destination: file path, -, ps:/path, ps:/prefix/, sm:id"`
	Raw    bool   `help:"Output raw value without expanding JSON (only for file/stdout destination)"`
	Format string `default:"dotenv" enum:"dotenv,export" help:"Output format for file/stdout destination (dotenv or export)"`
}

// Run executes the sync command.
func (c *SyncCmd) Run(appCtx *Context) error {
	entries, err := c.readEntries(appCtx)
	if err != nil {
		return fmt.Errorf("sync command failed: %w", err)
	}

	if err := c.writeEntries(appCtx, entries); err != nil {
		return fmt.Errorf("sync command failed: %w", err)
	}

	return nil
}

func isBackendRef(s string) bool {
	return strings.HasPrefix(s, "ps:") || strings.HasPrefix(s, "sm:")
}

func isPrefix(s string) bool {
	return strings.HasSuffix(s, "/")
}

func isStdio(s string) bool {
	return s == "-"
}

// readEntries reads entries from the source.
func (c *SyncCmd) readEntries(appCtx *Context) ([]dotenv.Entry, error) {
	ctx := context.Background()

	// File or stdin
	if !isBackendRef(c.From) {
		var r io.Reader
		if isStdio(c.From) {
			r = os.Stdin
		} else {
			f, err := os.Open(c.From)
			if err != nil {
				return nil, fmt.Errorf("open source file: %w", err)
			}
			defer f.Close()
			r = f
		}
		return dotenv.Parse(r)
	}

	// Backend ref
	ref, err := backend.ParseRef(c.From)
	if err != nil {
		return nil, fmt.Errorf("invalid source ref: %w", err)
	}

	b, err := appCtx.BackendFactory(ref.Type)
	if err != nil {
		return nil, fmt.Errorf("create backend: %w", err)
	}

	// PS or SM prefix (ends with /)
	if isPrefix(c.From) {
		results, err := b.GetByPrefix(ctx, ref.Path, backend.GetByPrefixOptions{Recursive: true})
		if err != nil {
			return nil, err
		}
		var entries []dotenv.Entry
		normalizedPrefix := strings.TrimRight(ref.Path, "/") + "/"
		for _, e := range results {
			relPath := strings.TrimPrefix(e.Path, normalizedPrefix)
			if relPath == "" || relPath == e.Path {
				continue
			}
			key := strings.ToUpper(relPath)
			key = strings.ReplaceAll(key, "/", "_")
			entries = append(entries, dotenv.Entry{Key: key, Value: e.Value})
		}
		sortEntries(entries)
		return entries, nil
	}

	// Single ref (ps:/path or sm:id)
	val, err := b.Get(ctx, c.From, backend.GetOptions{ForceRaw: true})
	if err != nil {
		return nil, err
	}

	// Raw mode: skip JSON expansion, return as single entry
	if c.Raw {
		keyName := path.Base(ref.Path)
		return []dotenv.Entry{{Key: keyName, Value: val}}, nil
	}

	// Try JSON parse
	var obj map[string]any
	if err := json.Unmarshal([]byte(val), &obj); err == nil {
		var entries []dotenv.Entry
		for k, v := range obj {
			entries = append(entries, dotenv.Entry{Key: k, Value: fmt.Sprintf("%v", v)})
		}
		sortEntries(entries)
		return entries, nil
	}

	// Scalar value
	keyName := path.Base(ref.Path)
	return []dotenv.Entry{{Key: keyName, Value: val}}, nil
}

// writeEntries writes entries to the destination.
func (c *SyncCmd) writeEntries(appCtx *Context, entries []dotenv.Entry) error {
	ctx := context.Background()

	// File or stdout
	if !isBackendRef(c.To) {
		var w io.Writer
		if isStdio(c.To) {
			w = os.Stdout
		} else {
			f, err := os.Create(c.To)
			if err != nil {
				return fmt.Errorf("create destination file: %w", err)
			}
			defer f.Close()
			w = f
		}

		writeFn := dotenv.Write
		if c.Format == "export" {
			writeFn = dotenv.WriteExport
		}

		if c.Raw {
			// Raw mode: output entries as-is
			return writeFn(w, entries)
		}

		// Non-raw: expand JSON values
		var expanded []dotenv.Entry
		for _, e := range entries {
			var obj map[string]any
			if err := json.Unmarshal([]byte(e.Value), &obj); err == nil {
				for k, v := range obj {
					expanded = append(expanded, dotenv.Entry{Key: k, Value: fmt.Sprintf("%v", v)})
				}
			} else {
				expanded = append(expanded, e)
			}
		}
		sortEntries(expanded)
		return writeFn(w, expanded)
	}

	// Backend ref
	ref, err := backend.ParseRef(c.To)
	if err != nil {
		return fmt.Errorf("invalid destination ref: %w", err)
	}

	b, err := appCtx.BackendFactory(ref.Type)
	if err != nil {
		return fmt.Errorf("create backend: %w", err)
	}

	// PS prefix (ends with /): write each entry as individual parameter
	if ref.Type == backend.BackendTypePS && isPrefix(c.To) {
		basePath := strings.TrimRight(ref.Path, "/")
		for _, e := range entries {
			key := strings.ToLower(e.Key)
			paramRef := fmt.Sprintf("ps:%s/%s", basePath, key)
			err := b.Put(ctx, paramRef, backend.PutOptions{
				Value:     e.Value,
				StoreMode: tags.StoreModeRaw,
			})
			if err != nil {
				return fmt.Errorf("put %s: %w", paramRef, err)
			}
		}
		return nil
	}

	// Single ref (ps:/path or sm:id): marshal entries to JSON
	jsonVal, err := entriesToJSON(entries)
	if err != nil {
		return fmt.Errorf("marshal entries: %w", err)
	}

	return b.Put(ctx, c.To, backend.PutOptions{
		Value:     jsonVal,
		StoreMode: tags.StoreModeJSON,
	})
}

// entriesToJSON converts entries to a JSON object string.
func entriesToJSON(entries []dotenv.Entry) (string, error) {
	m := make(map[string]string, len(entries))
	for _, e := range entries {
		m[e.Key] = e.Value
	}
	data, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// sortEntries sorts entries by key.
func sortEntries(entries []dotenv.Entry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Key < entries[j].Key
	})
}
