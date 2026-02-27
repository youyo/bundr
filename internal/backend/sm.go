package backend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/youyo/bundr/internal/tags"
)

// smClient abstracts the AWS Secrets Manager API for testing.
type smClient interface {
	CreateSecret(ctx context.Context, input *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error)
	PutSecretValue(ctx context.Context, input *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error)
	GetSecretValue(ctx context.Context, input *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
	DescribeSecret(ctx context.Context, input *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error)
	TagResource(ctx context.Context, input *secretsmanager.TagResourceInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.TagResourceOutput, error)
}

// SMBackend implements Backend for AWS Secrets Manager.
type SMBackend struct {
	client smClient
}

// NewSMBackend creates a new SMBackend with the given client.
func NewSMBackend(client smClient) *SMBackend {
	return &SMBackend{client: client}
}

// Put creates or updates a secret in AWS Secrets Manager.
func (b *SMBackend) Put(ctx context.Context, ref string, opts PutOptions) error {
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

	managedTags := tags.ManagedTags(opts.StoreMode)
	smTags := mapToSMTags(managedTags)

	// Try to create the secret first
	_, err := b.client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(ref),
		SecretString: aws.String(value),
		Tags:         smTags,
	})
	if err != nil {
		// If the secret already exists, update it
		var existsErr *smtypes.ResourceExistsException
		if !errors.As(err, &existsErr) {
			return fmt.Errorf("create secret: %w", err)
		}

		// Update the secret value
		_, err = b.client.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
			SecretId:     aws.String(ref),
			SecretString: aws.String(value),
		})
		if err != nil {
			return fmt.Errorf("put secret value: %w", err)
		}

		// Update tags
		_, err = b.client.TagResource(ctx, &secretsmanager.TagResourceInput{
			SecretId: aws.String(ref),
			Tags:     smTags,
		})
		if err != nil {
			return fmt.Errorf("tag resource: %w", err)
		}
	}

	return nil
}

// Get retrieves a secret value from AWS Secrets Manager.
func (b *SMBackend) Get(ctx context.Context, ref string, opts GetOptions) (string, error) {
	// Get the secret value
	result, err := b.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(ref),
	})
	if err != nil {
		return "", fmt.Errorf("get secret value: %w", err)
	}

	value := aws.ToString(result.SecretString)

	// ForceRaw: return as-is
	if opts.ForceRaw {
		return value, nil
	}

	// Read tags to determine store mode
	desc, err := b.client.DescribeSecret(ctx, &secretsmanager.DescribeSecretInput{
		SecretId: aws.String(ref),
	})
	if err != nil {
		return "", fmt.Errorf("describe secret: %w", err)
	}

	storeMode := getTagValue(desc.Tags, tags.TagStoreMode)

	// ForceJSON or storeMode=json: decode JSON
	if opts.ForceJSON || storeMode == tags.StoreModeJSON {
		return decodeJSON(value)
	}

	return value, nil
}

// mapToSMTags converts a map to Secrets Manager Tag slice.
func mapToSMTags(m map[string]string) []smtypes.Tag {
	result := make([]smtypes.Tag, 0, len(m))
	for k, v := range m {
		result = append(result, smtypes.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		})
	}
	return result
}

// GetByPrefix is not supported for Secrets Manager backend.
func (b *SMBackend) GetByPrefix(_ context.Context, _ string, _ GetByPrefixOptions) ([]ParameterEntry, error) {
	return nil, fmt.Errorf("GetByPrefix is not supported for Secrets Manager backend")
}

// getTagValue finds a tag value by key from a slice of SM tags.
func getTagValue(tagSlice []smtypes.Tag, key string) string {
	for _, t := range tagSlice {
		if aws.ToString(t.Key) == key {
			return aws.ToString(t.Value)
		}
	}
	return ""
}
