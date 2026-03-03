package backend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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
	ListSecrets(ctx context.Context, input *secretsmanager.ListSecretsInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error)
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
	parsed, err := ParseRef(ref)
	if err != nil {
		return err
	}
	secretName := parsed.Path

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
	_, createErr := b.client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(secretName),
		SecretString: aws.String(value),
		Tags:         smTags,
	})
	if createErr != nil {
		// If the secret already exists, update it
		var existsErr *smtypes.ResourceExistsException
		if !errors.As(createErr, &existsErr) {
			return fmt.Errorf("create secret: %w", createErr)
		}

		// Update the secret value
		_, err = b.client.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
			SecretId:     aws.String(secretName),
			SecretString: aws.String(value),
		})
		if err != nil {
			return fmt.Errorf("put secret value: %w", err)
		}

		// Update tags
		_, err = b.client.TagResource(ctx, &secretsmanager.TagResourceInput{
			SecretId: aws.String(secretName),
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
	parsed, err := ParseRef(ref)
	if err != nil {
		return "", err
	}
	secretName := parsed.Path

	// Get the secret value
	result, err := b.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
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
		SecretId: aws.String(secretName),
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

// GetByPrefix retrieves all secrets with the given name prefix from AWS Secrets Manager.
// An empty prefix returns all secrets.
func (b *SMBackend) GetByPrefix(ctx context.Context, prefix string, opts GetByPrefixOptions) ([]ParameterEntry, error) {
	var entries []ParameterEntry
	var nextToken *string

	for {
		input := &secretsmanager.ListSecretsInput{}
		if prefix != "" {
			input.Filters = []smtypes.Filter{
				{Key: smtypes.FilterNameStringTypeName, Values: []string{prefix}},
			}
		}
		if nextToken != nil {
			input.NextToken = nextToken
		}

		out, err := b.client.ListSecrets(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("list secrets: %w", err)
		}

		for _, secret := range out.SecretList {
			name := aws.ToString(secret.Name)

			if !opts.Recursive && prefix != "" {
				remainder := strings.TrimPrefix(name, prefix)
				if strings.Contains(remainder, "/") {
					continue
				}
			}

			storeMode := getTagValue(secret.Tags, tags.TagStoreMode)

			var metadata map[string]any
			if opts.IncludeMetadata {
				metadata = map[string]any{
					"ARN":              aws.ToString(secret.ARN),
					"Name":             name,
					"Description":      aws.ToString(secret.Description),
					"CreatedDate":      secret.CreatedDate,
					"LastAccessedDate": secret.LastAccessedDate,
					"LastChangedDate":  secret.LastChangedDate,
					"LastRotatedDate":  secret.LastRotatedDate,
				}
			}

			entries = append(entries, ParameterEntry{
				Path:      name,
				Value:     "",
				StoreMode: storeMode,
				Metadata:  metadata,
			})
		}

		nextToken = out.NextToken
		if nextToken == nil {
			break
		}
	}

	return entries, nil
}

// Describe returns metadata for the given Secrets Manager ref as a map.
// Combines GetSecretValue (ARN, Name, VersionId, VersionStages, Value) and
// DescribeSecret (CreatedDate, LastAccessedDate, LastRotatedDate).
func (b *SMBackend) Describe(ctx context.Context, ref string) (map[string]any, error) {
	parsed, err := ParseRef(ref)
	if err != nil {
		return nil, err
	}
	secretName := parsed.Path

	gsvOut, err := b.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		return nil, fmt.Errorf("get secret value: %w", err)
	}

	result := map[string]any{
		"ARN":           aws.ToString(gsvOut.ARN),
		"Name":          aws.ToString(gsvOut.Name),
		"VersionId":     aws.ToString(gsvOut.VersionId),
		"VersionStages": gsvOut.VersionStages,
		"Value":         aws.ToString(gsvOut.SecretString),
	}

	dsOut, err := b.client.DescribeSecret(ctx, &secretsmanager.DescribeSecretInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		return nil, fmt.Errorf("describe secret: %w", err)
	}

	result["CreatedDate"] = dsOut.CreatedDate
	result["LastAccessedDate"] = dsOut.LastAccessedDate
	result["LastRotatedDate"] = dsOut.LastRotatedDate

	return result, nil
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
