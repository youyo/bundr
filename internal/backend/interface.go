package backend

import "context"

// PutOptions contains options for the Put operation.
type PutOptions struct {
	Value     string
	StoreMode string // "raw" or "json"
	ValueType string // "string" or "secure"
	KMSKeyID  string
	Tags      map[string]string
}

// GetOptions contains options for the Get operation.
type GetOptions struct {
	ForceRaw  bool
	ForceJSON bool
}

// Backend is the interface for interacting with AWS parameter/secret backends.
type Backend interface {
	Put(ctx context.Context, ref string, opts PutOptions) error
	Get(ctx context.Context, ref string, opts GetOptions) (string, error)
}
