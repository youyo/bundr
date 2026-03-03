package backend

import "context"

// PutOptions contains options for the Put operation.
type PutOptions struct {
	Value        string
	StoreMode    string // "raw" or "json"
	ValueType    string // "string" or "secure"
	KMSKeyID     string
	Tags         map[string]string
	AdvancedTier bool // true to force Advanced tier (equivalent to psa: prefix)
	TierExplicit bool // true when --tier flag was explicitly specified (skips auto-detect)
}

// GetOptions contains options for the Get operation.
type GetOptions struct {
	ForceRaw  bool
	ForceJSON bool
}

// GetByPrefixOptions contains options for the GetByPrefix operation.
type GetByPrefixOptions struct {
	Recursive       bool
	SkipTagFetch    bool // タグ取得スキップ（補完・cache refresh 専用）。StoreMode = ""
	IncludeMetadata bool // ls --describe 用: AWS レスポンスのメタデータを Metadata フィールドに格納
}

// ParameterEntry represents a single parameter retrieved by GetByPrefix.
type ParameterEntry struct {
	Path      string
	Value     string
	StoreMode string
	Metadata  map[string]any // nil = 未取得（IncludeMetadata=false 時）
}

// Backend is the interface for interacting with AWS parameter/secret backends.
type Backend interface {
	Put(ctx context.Context, ref string, opts PutOptions) error
	Get(ctx context.Context, ref string, opts GetOptions) (string, error)
	GetByPrefix(ctx context.Context, prefix string, opts GetByPrefixOptions) ([]ParameterEntry, error)
	Describe(ctx context.Context, ref string) (map[string]any, error)
}
