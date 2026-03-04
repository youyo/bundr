package cmd

import (
	"context"
	"fmt"
	"path"
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
// Used by ExecCmd.
func buildVars(ctx context.Context, appCtx *Context, opts VarsBuildOptions) (map[string]string, error) {
	ref, err := backend.ParseRef(opts.From)
	if err != nil {
		return nil, fmt.Errorf("invalid ref: %w", err)
	}

	if ref.Type == backend.BackendTypeSM {
		return nil, fmt.Errorf("sm: backend is not supported (use ps:)")
	}

	b, err := appCtx.BackendFactory(ref.Type)
	if err != nil {
		return nil, fmt.Errorf("create backend: %w", err)
	}

	entries, err := b.GetByPrefix(ctx, ref.Path, backend.GetByPrefixOptions{Recursive: true})
	if err != nil {
		return nil, err
	}

	if appCtx.CacheStore != nil {
		_ = appCtx.CacheStore.Write(string(ref.Type), toCacheEntries(entries))
	}

	flatOpts := flatten.Options{
		Delimiter:      opts.FlattenDelim,
		ArrayMode:      opts.ArrayMode,
		ArrayJoinDelim: opts.ArrayJoinDelim,
		Upper:          opts.Upper,
		NoFlatten:      opts.NoFlatten,
	}

	vars := make(map[string]string)

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

// pathToKey converts an SSM path to a key name by trimming the from prefix.
func pathToKey(paramPath, fromPath, delim string) string {
	trimmed := strings.TrimPrefix(paramPath, strings.TrimRight(fromPath, "/")+"/")
	return strings.ReplaceAll(trimmed, "/", delim)
}
