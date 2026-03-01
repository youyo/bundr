package backend

import (
	"context"
	"time"
)

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

// GetByPrefixOptions contains options for the GetByPrefix operation.
type GetByPrefixOptions struct {
	Recursive bool
}

// ParameterEntry represents a single parameter retrieved by GetByPrefix.
type ParameterEntry struct {
	Path      string
	Value     string
	StoreMode string
}

// DescribeOutput holds metadata about a parameter or secret.
type DescribeOutput struct {
	Path             string
	ARN              string
	Version          int64
	LastModifiedDate *time.Time
	Tags             map[string]string

	// PS-specific
	ParameterType string // "String" or "SecureString"
	Tier          string // "Standard" or "Advanced"
	DataType      string

	// SM-specific
	CreatedDate *time.Time
}

// Backend is the interface for interacting with AWS parameter/secret backends.
type Backend interface {
	Put(ctx context.Context, ref string, opts PutOptions) error
	Get(ctx context.Context, ref string, opts GetOptions) (string, error)
	GetByPrefix(ctx context.Context, prefix string, opts GetByPrefixOptions) ([]ParameterEntry, error)
	Describe(ctx context.Context, ref string) (*DescribeOutput, error)
}
