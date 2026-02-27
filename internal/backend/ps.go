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

	input := &ssm.PutParameterInput{
		Name:      aws.String(parsed.Path),
		Value:     aws.String(value),
		Type:      paramType,
		Overwrite: aws.Bool(true),
		Tags:      ssmTags,
	}

	// Set tier for Advanced parameters
	if parsed.Type == BackendTypePSA {
		input.Tier = ssmtypes.ParameterTierAdvanced
	}

	// Set KMS key if specified
	if opts.KMSKeyID != "" {
		input.KeyId = aws.String(opts.KMSKeyID)
	}

	_, err = b.client.PutParameter(ctx, input)
	if err != nil {
		return fmt.Errorf("ssm PutParameter: %w", err)
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
