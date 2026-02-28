package backend

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/youyo/bundr/internal/tags"
)

// SSMClient is the interface for the SSM API operations used by PSBackend.
type SSMClient interface {
	PutParameter(ctx context.Context, input *ssm.PutParameterInput, optFns ...func(*ssm.Options)) (*ssm.PutParameterOutput, error)
	GetParameter(ctx context.Context, input *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
	GetParametersByPath(ctx context.Context, input *ssm.GetParametersByPathInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error)
	AddTagsToResource(ctx context.Context, input *ssm.AddTagsToResourceInput, optFns ...func(*ssm.Options)) (*ssm.AddTagsToResourceOutput, error)
	ListTagsForResource(ctx context.Context, input *ssm.ListTagsForResourceInput, optFns ...func(*ssm.Options)) (*ssm.ListTagsForResourceOutput, error)
}

// PSBackend implements Backend for SSM Parameter Store (ps: and psa: refs).
type PSBackend struct {
	client SSMClient
}

// NewPSBackend creates a new PSBackend with the given SSM client.
func NewPSBackend(client SSMClient) *PSBackend {
	return &PSBackend{client: client}
}

// Put stores a parameter in SSM Parameter Store.
func (b *PSBackend) Put(ctx context.Context, ref string, opts PutOptions) error {
	parsed, err := ParseRef(ref)
	if err != nil {
		return err
	}

	value := opts.Value

	// JSON mode: encode scalar values
	if opts.StoreMode == tags.StoreModeJSON {
		if !json.Valid([]byte(opts.Value)) {
			encoded, err := json.Marshal(opts.Value)
			if err != nil {
				return fmt.Errorf("json encode: %w", err)
			}
			value = string(encoded)
		}
	}

	// Determine parameter type
	paramType := ssmtypes.ParameterTypeString
	if opts.ValueType == "secure" {
		paramType = ssmtypes.ParameterTypeSecureString
	}

	// Build managed tags
	managedTags := tags.ManagedTags(opts.StoreMode)
	for k, v := range opts.Tags {
		managedTags[k] = v
	}

	ssmTags := make([]ssmtypes.Tag, 0, len(managedTags))
	for k, v := range managedTags {
		ssmTags = append(ssmTags, ssmtypes.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		})
	}

	// Step 1: PutParameter WITHOUT Tags (Overwrite=true and Tags cannot be used together)
	input := &ssm.PutParameterInput{
		Name:      aws.String(parsed.Path),
		Value:     aws.String(value),
		Type:      paramType,
		Overwrite: aws.Bool(true),
	}

	// Set tier for Advanced parameters
	if parsed.Type == BackendTypePSA {
		input.Tier = ssmtypes.ParameterTierAdvanced
	}

	// Set KMS key if specified
	if opts.KMSKeyID != "" {
		input.KeyId = aws.String(opts.KMSKeyID)
	}

	if _, err = b.client.PutParameter(ctx, input); err != nil {
		return fmt.Errorf("ssm PutParameter: %w", err)
	}

	// Step 2: AddTagsToResource to set managed tags separately
	if _, err = b.client.AddTagsToResource(ctx, &ssm.AddTagsToResourceInput{
		ResourceType: ssmtypes.ResourceTypeForTaggingParameter,
		ResourceId:   aws.String(parsed.Path),
		Tags:         ssmTags,
	}); err != nil {
		return fmt.Errorf("ssm AddTagsToResource failed (parameter was saved but tags are missing; run 'bundr put' again to retry): %w", err)
	}

	return nil
}

// Get retrieves a parameter from SSM Parameter Store.
func (b *PSBackend) Get(ctx context.Context, ref string, opts GetOptions) (string, error) {
	parsed, err := ParseRef(ref)
	if err != nil {
		return "", err
	}

	// Get the parameter value
	getOutput, err := b.client.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(parsed.Path),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return "", fmt.Errorf("ssm GetParameter: %w", err)
	}

	rawValue := aws.ToString(getOutput.Parameter.Value)

	// ForceRaw: return as-is
	if opts.ForceRaw {
		return rawValue, nil
	}

	// ForceJSON: always decode
	if opts.ForceJSON {
		return decodeJSON(rawValue)
	}

	// Check tags to determine store mode
	tagsOutput, err := b.client.ListTagsForResource(ctx, &ssm.ListTagsForResourceInput{
		ResourceId:   aws.String(parsed.Path),
		ResourceType: ssmtypes.ResourceTypeForTaggingParameter,
	})
	if err != nil {
		return "", fmt.Errorf("ssm ListTagsForResource: %w", err)
	}

	storeMode := tags.StoreModeRaw
	for _, tag := range tagsOutput.TagList {
		if aws.ToString(tag.Key) == tags.TagStoreMode {
			storeMode = aws.ToString(tag.Value)
			break
		}
	}

	if storeMode == tags.StoreModeJSON {
		return decodeJSON(rawValue)
	}

	return rawValue, nil
}

// GetByPrefix retrieves all parameters under the given SSM path prefix.
func (b *PSBackend) GetByPrefix(ctx context.Context, prefix string, opts GetByPrefixOptions) ([]ParameterEntry, error) {
	var entries []ParameterEntry
	var nextToken *string

	for {
		input := &ssm.GetParametersByPathInput{
			Path:           aws.String(prefix),
			WithDecryption: aws.Bool(true),
			Recursive:      aws.Bool(opts.Recursive),
		}
		if nextToken != nil {
			input.NextToken = nextToken
		}

		out, err := b.client.GetParametersByPath(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("ssm GetParametersByPath: %w", err)
		}

		for _, param := range out.Parameters {
			path := aws.ToString(param.Name)
			value := aws.ToString(param.Value)

			storeMode, err := b.getStoreMode(ctx, path)
			if err != nil {
				return nil, fmt.Errorf("get store mode for %s: %w", path, err)
			}

			entries = append(entries, ParameterEntry{
				Path:      path,
				Value:     value,
				StoreMode: storeMode,
			})
		}

		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}

	return entries, nil
}

// getStoreMode retrieves the cli-store-mode tag for the given SSM parameter path.
func (b *PSBackend) getStoreMode(ctx context.Context, path string) (string, error) {
	tagsOut, err := b.client.ListTagsForResource(ctx, &ssm.ListTagsForResourceInput{
		ResourceId:   aws.String(path),
		ResourceType: ssmtypes.ResourceTypeForTaggingParameter,
	})
	if err != nil {
		return "", err
	}

	for _, tag := range tagsOut.TagList {
		if aws.ToString(tag.Key) == tags.TagStoreMode {
			return aws.ToString(tag.Value), nil
		}
	}
	return tags.StoreModeRaw, nil
}
