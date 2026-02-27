package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/youyo/bundr/internal/tags"
)

// mockSMClient is a mock implementation of the smClient interface for testing.
type mockSMClient struct {
	secrets map[string]*mockSecret
}

type mockSecret struct {
	value string
	tags  []smtypes.Tag
}

func newMockSMClient() *mockSMClient {
	return &mockSMClient{
		secrets: make(map[string]*mockSecret),
	}
}

func (m *mockSMClient) CreateSecret(ctx context.Context, input *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
	name := aws.ToString(input.Name)
	if _, exists := m.secrets[name]; exists {
		return nil, &smtypes.ResourceExistsException{Message: aws.String("already exists")}
	}
	m.secrets[name] = &mockSecret{
		value: aws.ToString(input.SecretString),
		tags:  input.Tags,
	}
	return &secretsmanager.CreateSecretOutput{
		Name: input.Name,
	}, nil
}

func (m *mockSMClient) PutSecretValue(ctx context.Context, input *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error) {
	name := aws.ToString(input.SecretId)
	secret, exists := m.secrets[name]
	if !exists {
		return nil, fmt.Errorf("secret not found: %s", name)
	}
	secret.value = aws.ToString(input.SecretString)
	return &secretsmanager.PutSecretValueOutput{}, nil
}

func (m *mockSMClient) GetSecretValue(ctx context.Context, input *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	name := aws.ToString(input.SecretId)
	secret, exists := m.secrets[name]
	if !exists {
		return nil, fmt.Errorf("secret not found: %s", name)
	}
	return &secretsmanager.GetSecretValueOutput{
		SecretString: aws.String(secret.value),
		Name:         aws.String(name),
	}, nil
}

func (m *mockSMClient) DescribeSecret(ctx context.Context, input *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
	name := aws.ToString(input.SecretId)
	secret, exists := m.secrets[name]
	if !exists {
		return nil, fmt.Errorf("secret not found: %s", name)
	}
	return &secretsmanager.DescribeSecretOutput{
		Tags: secret.tags,
		Name: aws.String(name),
	}, nil
}

func (m *mockSMClient) TagResource(ctx context.Context, input *secretsmanager.TagResourceInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.TagResourceOutput, error) {
	name := aws.ToString(input.SecretId)
	secret, exists := m.secrets[name]
	if !exists {
		return nil, fmt.Errorf("secret not found: %s", name)
	}
	// Merge tags: update existing keys, add new ones
	for _, newTag := range input.Tags {
		found := false
		for i, existingTag := range secret.tags {
			if aws.ToString(existingTag.Key) == aws.ToString(newTag.Key) {
				secret.tags[i] = newTag
				found = true
				break
			}
		}
		if !found {
			secret.tags = append(secret.tags, newTag)
		}
	}
	return &secretsmanager.TagResourceOutput{}, nil
}

func TestSMBackend_PutCreateSecret(t *testing.T) {
	ctx := context.Background()
	client := newMockSMClient()
	b := NewSMBackend(client)

	err := b.Put(ctx, "my-secret", PutOptions{
		Value:     "hello",
		StoreMode: tags.StoreModeRaw,
	})
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	// Verify secret was created with correct value
	secret := client.secrets["my-secret"]
	if secret == nil {
		t.Fatal("secret not created")
	}
	if secret.value != "hello" {
		t.Errorf("stored value = %q, want %q", secret.value, "hello")
	}

	// Verify tags
	tagMap := tagsToMap(secret.tags)
	if tagMap[tags.TagCLI] != tags.TagCLIValue {
		t.Errorf("tag %s = %q, want %q", tags.TagCLI, tagMap[tags.TagCLI], tags.TagCLIValue)
	}
	if tagMap[tags.TagStoreMode] != tags.StoreModeRaw {
		t.Errorf("tag %s = %q, want %q", tags.TagStoreMode, tagMap[tags.TagStoreMode], tags.StoreModeRaw)
	}
	if tagMap[tags.TagSchema] != tags.TagSchemaValue {
		t.Errorf("tag %s = %q, want %q", tags.TagSchema, tagMap[tags.TagSchema], tags.TagSchemaValue)
	}
}

func TestSMBackend_PutJSONScalar(t *testing.T) {
	ctx := context.Background()
	client := newMockSMClient()
	b := NewSMBackend(client)

	err := b.Put(ctx, "my-secret", PutOptions{
		Value:     "hello",
		StoreMode: tags.StoreModeJSON,
	})
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	// JSON-encoded scalar: "hello" → "\"hello\""
	secret := client.secrets["my-secret"]
	if secret.value != `"hello"` {
		t.Errorf("stored value = %q, want %q", secret.value, `"hello"`)
	}
}

func TestSMBackend_PutJSONObject(t *testing.T) {
	ctx := context.Background()
	client := newMockSMClient()
	b := NewSMBackend(client)

	err := b.Put(ctx, "my-secret", PutOptions{
		Value:     `{"key":"value"}`,
		StoreMode: tags.StoreModeJSON,
	})
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	// Valid JSON should be stored as-is
	secret := client.secrets["my-secret"]
	if secret.value != `{"key":"value"}` {
		t.Errorf("stored value = %q, want %q", secret.value, `{"key":"value"}`)
	}
}

func TestSMBackend_PutUpdateExisting(t *testing.T) {
	ctx := context.Background()
	client := newMockSMClient()
	b := NewSMBackend(client)

	// First put creates the secret
	err := b.Put(ctx, "my-secret", PutOptions{
		Value:     "v1",
		StoreMode: tags.StoreModeRaw,
	})
	if err != nil {
		t.Fatalf("first Put() error: %v", err)
	}

	// Second put should update (not fail)
	err = b.Put(ctx, "my-secret", PutOptions{
		Value:     "v2",
		StoreMode: tags.StoreModeJSON,
	})
	if err != nil {
		t.Fatalf("second Put() error: %v", err)
	}

	secret := client.secrets["my-secret"]
	// v2 in JSON mode should be stored as "\"v2\""
	want := `"v2"`
	if secret.value != want {
		t.Errorf("stored value = %q, want %q", secret.value, want)
	}

	// Tags should be updated
	tagMap := tagsToMap(secret.tags)
	if tagMap[tags.TagStoreMode] != tags.StoreModeJSON {
		t.Errorf("tag %s = %q, want %q", tags.TagStoreMode, tagMap[tags.TagStoreMode], tags.StoreModeJSON)
	}
}

func TestSMBackend_GetRaw(t *testing.T) {
	ctx := context.Background()
	client := newMockSMClient()
	b := NewSMBackend(client)

	// Pre-populate with raw value
	err := b.Put(ctx, "my-secret", PutOptions{
		Value:     "plain-text",
		StoreMode: tags.StoreModeRaw,
	})
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	val, err := b.Get(ctx, "my-secret", GetOptions{})
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if val != "plain-text" {
		t.Errorf("Get() = %q, want %q", val, "plain-text")
	}
}

func TestSMBackend_GetJSONDecode(t *testing.T) {
	ctx := context.Background()
	client := newMockSMClient()
	b := NewSMBackend(client)

	// Put JSON scalar
	err := b.Put(ctx, "my-secret", PutOptions{
		Value:     "hello",
		StoreMode: tags.StoreModeJSON,
	})
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	// Get should auto-decode based on cli-store-mode tag
	val, err := b.Get(ctx, "my-secret", GetOptions{})
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if val != "hello" {
		t.Errorf("Get() = %q, want %q", val, "hello")
	}
}

func TestSMBackend_GetJSONObject(t *testing.T) {
	ctx := context.Background()
	client := newMockSMClient()
	b := NewSMBackend(client)

	err := b.Put(ctx, "my-secret", PutOptions{
		Value:     `{"key":"value"}`,
		StoreMode: tags.StoreModeJSON,
	})
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	val, err := b.Get(ctx, "my-secret", GetOptions{})
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}

	// JSON object should be returned as-is
	var obj map[string]string
	if err := json.Unmarshal([]byte(val), &obj); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if obj["key"] != "value" {
		t.Errorf("Get() parsed key = %q, want %q", obj["key"], "value")
	}
}

func TestSMBackend_GetForceRaw(t *testing.T) {
	ctx := context.Background()
	client := newMockSMClient()
	b := NewSMBackend(client)

	err := b.Put(ctx, "my-secret", PutOptions{
		Value:     "hello",
		StoreMode: tags.StoreModeJSON,
	})
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	// ForceRaw should return the raw stored value (JSON-encoded)
	val, err := b.Get(ctx, "my-secret", GetOptions{ForceRaw: true})
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if val != `"hello"` {
		t.Errorf("Get(ForceRaw) = %q, want %q", val, `"hello"`)
	}
}

func TestSMBackend_GetForceJSON(t *testing.T) {
	ctx := context.Background()
	client := newMockSMClient()
	b := NewSMBackend(client)

	// Store raw but with JSON-encoded content
	err := b.Put(ctx, "my-secret", PutOptions{
		Value:     `"hello"`,
		StoreMode: tags.StoreModeRaw,
	})
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	// ForceJSON should decode regardless of tag
	val, err := b.Get(ctx, "my-secret", GetOptions{ForceJSON: true})
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if val != "hello" {
		t.Errorf("Get(ForceJSON) = %q, want %q", val, "hello")
	}
}

func TestSMBackend_GetNotFound(t *testing.T) {
	ctx := context.Background()
	client := newMockSMClient()
	b := NewSMBackend(client)

	_, err := b.Get(ctx, "nonexistent", GetOptions{})
	if err == nil {
		t.Error("Get() expected error for nonexistent secret, got nil")
	}
}

// tagsToMap converts a slice of SM tags to a map for easy lookup.
func tagsToMap(tagSlice []smtypes.Tag) map[string]string {
	m := make(map[string]string, len(tagSlice))
	for _, t := range tagSlice {
		m[aws.ToString(t.Key)] = aws.ToString(t.Value)
	}
	return m
}
