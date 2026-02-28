package backend

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/youyo/bundr/internal/tags"
)

// mockSSMClient is a test double for the SSM API.
type mockSSMClient struct {
	putParameterFn          func(ctx context.Context, input *ssm.PutParameterInput, optFns ...func(*ssm.Options)) (*ssm.PutParameterOutput, error)
	getParameterFn          func(ctx context.Context, input *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
	getParametersByPathFn   func(ctx context.Context, input *ssm.GetParametersByPathInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error)
	addTagsToResourceFn     func(ctx context.Context, input *ssm.AddTagsToResourceInput, optFns ...func(*ssm.Options)) (*ssm.AddTagsToResourceOutput, error)
	listTagsForResourceFn   func(ctx context.Context, input *ssm.ListTagsForResourceInput, optFns ...func(*ssm.Options)) (*ssm.ListTagsForResourceOutput, error)

	// Call recording fields for verifying call sequences
	putParameterCalls      []*ssm.PutParameterInput
	addTagsToResourceCalls []*ssm.AddTagsToResourceInput
}

func (m *mockSSMClient) PutParameter(ctx context.Context, input *ssm.PutParameterInput, optFns ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
	m.putParameterCalls = append(m.putParameterCalls, input)
	return m.putParameterFn(ctx, input, optFns...)
}

func (m *mockSSMClient) GetParameter(ctx context.Context, input *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	return m.getParameterFn(ctx, input, optFns...)
}

func (m *mockSSMClient) GetParametersByPath(ctx context.Context, input *ssm.GetParametersByPathInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
	if m.getParametersByPathFn == nil {
		return &ssm.GetParametersByPathOutput{}, nil
	}
	return m.getParametersByPathFn(ctx, input, optFns...)
}

func (m *mockSSMClient) AddTagsToResource(ctx context.Context, input *ssm.AddTagsToResourceInput, optFns ...func(*ssm.Options)) (*ssm.AddTagsToResourceOutput, error) {
	m.addTagsToResourceCalls = append(m.addTagsToResourceCalls, input)
	return m.addTagsToResourceFn(ctx, input, optFns...)
}

func (m *mockSSMClient) ListTagsForResource(ctx context.Context, input *ssm.ListTagsForResourceInput, optFns ...func(*ssm.Options)) (*ssm.ListTagsForResourceOutput, error) {
	return m.listTagsForResourceFn(ctx, input, optFns...)
}

func TestPSBackend_PutRaw(t *testing.T) {
	ctx := context.Background()
	var capturedInput *ssm.PutParameterInput

	client := &mockSSMClient{
		putParameterFn: func(_ context.Context, input *ssm.PutParameterInput, _ ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
			capturedInput = input
			return &ssm.PutParameterOutput{}, nil
		},
		addTagsToResourceFn: func(_ context.Context, _ *ssm.AddTagsToResourceInput, _ ...func(*ssm.Options)) (*ssm.AddTagsToResourceOutput, error) {
			return &ssm.AddTagsToResourceOutput{}, nil
		},
	}

	backend := NewPSBackend(client)
	err := backend.Put(ctx, "ps:/app/test/KEY", PutOptions{
		Value:     "hello",
		StoreMode: tags.StoreModeRaw,
	})
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	if capturedInput == nil {
		t.Fatal("PutParameter was not called")
	}
	if aws.ToString(capturedInput.Name) != "/app/test/KEY" {
		t.Errorf("Name = %q, want %q", aws.ToString(capturedInput.Name), "/app/test/KEY")
	}
	if aws.ToString(capturedInput.Value) != "hello" {
		t.Errorf("Value = %q, want %q", aws.ToString(capturedInput.Value), "hello")
	}
	if capturedInput.Type != ssmtypes.ParameterTypeString {
		t.Errorf("Type = %v, want %v", capturedInput.Type, ssmtypes.ParameterTypeString)
	}
}

func TestPSBackend_PutJSONScalar(t *testing.T) {
	ctx := context.Background()
	var capturedInput *ssm.PutParameterInput

	client := &mockSSMClient{
		putParameterFn: func(_ context.Context, input *ssm.PutParameterInput, _ ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
			capturedInput = input
			return &ssm.PutParameterOutput{}, nil
		},
		addTagsToResourceFn: func(_ context.Context, _ *ssm.AddTagsToResourceInput, _ ...func(*ssm.Options)) (*ssm.AddTagsToResourceOutput, error) {
			return &ssm.AddTagsToResourceOutput{}, nil
		},
	}

	backend := NewPSBackend(client)
	err := backend.Put(ctx, "ps:/app/test/KEY", PutOptions{
		Value:     "hello",
		StoreMode: tags.StoreModeJSON,
	})
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	// Scalar "hello" should be JSON-encoded to "\"hello\""
	if aws.ToString(capturedInput.Value) != `"hello"` {
		t.Errorf("Value = %q, want %q", aws.ToString(capturedInput.Value), `"hello"`)
	}
}

func TestPSBackend_PutJSONObject(t *testing.T) {
	ctx := context.Background()
	var capturedInput *ssm.PutParameterInput

	client := &mockSSMClient{
		putParameterFn: func(_ context.Context, input *ssm.PutParameterInput, _ ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
			capturedInput = input
			return &ssm.PutParameterOutput{}, nil
		},
		addTagsToResourceFn: func(_ context.Context, _ *ssm.AddTagsToResourceInput, _ ...func(*ssm.Options)) (*ssm.AddTagsToResourceOutput, error) {
			return &ssm.AddTagsToResourceOutput{}, nil
		},
	}

	backend := NewPSBackend(client)
	err := backend.Put(ctx, "ps:/app/test/KEY", PutOptions{
		Value:     `{"key":"value"}`,
		StoreMode: tags.StoreModeJSON,
	})
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	// JSON object should be stored as-is
	if aws.ToString(capturedInput.Value) != `{"key":"value"}` {
		t.Errorf("Value = %q, want %q", aws.ToString(capturedInput.Value), `{"key":"value"}`)
	}
}

func TestPSBackend_PutSecureString(t *testing.T) {
	ctx := context.Background()
	var capturedInput *ssm.PutParameterInput

	client := &mockSSMClient{
		putParameterFn: func(_ context.Context, input *ssm.PutParameterInput, _ ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
			capturedInput = input
			return &ssm.PutParameterOutput{}, nil
		},
		addTagsToResourceFn: func(_ context.Context, _ *ssm.AddTagsToResourceInput, _ ...func(*ssm.Options)) (*ssm.AddTagsToResourceOutput, error) {
			return &ssm.AddTagsToResourceOutput{}, nil
		},
	}

	backend := NewPSBackend(client)
	err := backend.Put(ctx, "ps:/app/test/SECRET", PutOptions{
		Value:     "secret-value",
		StoreMode: tags.StoreModeRaw,
		ValueType: "secure",
		KMSKeyID:  "alias/my-key",
	})
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	if capturedInput.Type != ssmtypes.ParameterTypeSecureString {
		t.Errorf("Type = %v, want %v", capturedInput.Type, ssmtypes.ParameterTypeSecureString)
	}
	if aws.ToString(capturedInput.KeyId) != "alias/my-key" {
		t.Errorf("KeyId = %q, want %q", aws.ToString(capturedInput.KeyId), "alias/my-key")
	}
}

func TestPSBackend_PutAdvancedTier(t *testing.T) {
	ctx := context.Background()
	var capturedInput *ssm.PutParameterInput

	client := &mockSSMClient{
		putParameterFn: func(_ context.Context, input *ssm.PutParameterInput, _ ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
			capturedInput = input
			return &ssm.PutParameterOutput{}, nil
		},
		addTagsToResourceFn: func(_ context.Context, _ *ssm.AddTagsToResourceInput, _ ...func(*ssm.Options)) (*ssm.AddTagsToResourceOutput, error) {
			return &ssm.AddTagsToResourceOutput{}, nil
		},
	}

	backend := NewPSBackend(client)
	err := backend.Put(ctx, "psa:/app/test/KEY", PutOptions{
		Value:     "hello",
		StoreMode: tags.StoreModeRaw,
	})
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	if capturedInput.Tier != ssmtypes.ParameterTierAdvanced {
		t.Errorf("Tier = %v, want %v", capturedInput.Tier, ssmtypes.ParameterTierAdvanced)
	}
}

func TestPSBackend_PutTags(t *testing.T) {
	ctx := context.Background()
	var capturedTags []ssmtypes.Tag

	client := &mockSSMClient{
		putParameterFn: func(_ context.Context, _ *ssm.PutParameterInput, _ ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
			return &ssm.PutParameterOutput{}, nil
		},
		addTagsToResourceFn: func(_ context.Context, input *ssm.AddTagsToResourceInput, _ ...func(*ssm.Options)) (*ssm.AddTagsToResourceOutput, error) {
			capturedTags = input.Tags
			return &ssm.AddTagsToResourceOutput{}, nil
		},
	}

	backend := NewPSBackend(client)
	err := backend.Put(ctx, "ps:/app/test/KEY", PutOptions{
		Value:     "hello",
		StoreMode: tags.StoreModeRaw,
	})
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	// Should have the 3 managed tags via AddTagsToResource
	tagMap := make(map[string]string)
	for _, tag := range capturedTags {
		tagMap[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

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

func TestPSBackend_GetRaw(t *testing.T) {
	ctx := context.Background()

	client := &mockSSMClient{
		getParameterFn: func(_ context.Context, input *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
			return &ssm.GetParameterOutput{
				Parameter: &ssmtypes.Parameter{
					Name:  input.Name,
					Value: aws.String("hello"),
				},
			}, nil
		},
		listTagsForResourceFn: func(_ context.Context, _ *ssm.ListTagsForResourceInput, _ ...func(*ssm.Options)) (*ssm.ListTagsForResourceOutput, error) {
			return &ssm.ListTagsForResourceOutput{
				TagList: []ssmtypes.Tag{
					{Key: aws.String(tags.TagCLI), Value: aws.String(tags.TagCLIValue)},
					{Key: aws.String(tags.TagStoreMode), Value: aws.String(tags.StoreModeRaw)},
					{Key: aws.String(tags.TagSchema), Value: aws.String(tags.TagSchemaValue)},
				},
			}, nil
		},
	}

	backend := NewPSBackend(client)
	val, err := backend.Get(ctx, "ps:/app/test/KEY", GetOptions{})
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if val != "hello" {
		t.Errorf("Get() = %q, want %q", val, "hello")
	}
}

func TestPSBackend_GetJSON(t *testing.T) {
	ctx := context.Background()

	client := &mockSSMClient{
		getParameterFn: func(_ context.Context, input *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
			return &ssm.GetParameterOutput{
				Parameter: &ssmtypes.Parameter{
					Name:  input.Name,
					Value: aws.String(`"hello"`), // JSON-encoded string
				},
			}, nil
		},
		listTagsForResourceFn: func(_ context.Context, _ *ssm.ListTagsForResourceInput, _ ...func(*ssm.Options)) (*ssm.ListTagsForResourceOutput, error) {
			return &ssm.ListTagsForResourceOutput{
				TagList: []ssmtypes.Tag{
					{Key: aws.String(tags.TagCLI), Value: aws.String(tags.TagCLIValue)},
					{Key: aws.String(tags.TagStoreMode), Value: aws.String(tags.StoreModeJSON)},
					{Key: aws.String(tags.TagSchema), Value: aws.String(tags.TagSchemaValue)},
				},
			}, nil
		},
	}

	backend := NewPSBackend(client)
	val, err := backend.Get(ctx, "ps:/app/test/KEY", GetOptions{})
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if val != "hello" {
		t.Errorf("Get() = %q, want %q", val, "hello")
	}
}

func TestPSBackend_GetForceRaw(t *testing.T) {
	ctx := context.Background()

	client := &mockSSMClient{
		getParameterFn: func(_ context.Context, input *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
			return &ssm.GetParameterOutput{
				Parameter: &ssmtypes.Parameter{
					Name:  input.Name,
					Value: aws.String(`"hello"`),
				},
			}, nil
		},
		listTagsForResourceFn: func(_ context.Context, _ *ssm.ListTagsForResourceInput, _ ...func(*ssm.Options)) (*ssm.ListTagsForResourceOutput, error) {
			return &ssm.ListTagsForResourceOutput{
				TagList: []ssmtypes.Tag{
					{Key: aws.String(tags.TagStoreMode), Value: aws.String(tags.StoreModeJSON)},
				},
			}, nil
		},
	}

	backend := NewPSBackend(client)
	val, err := backend.Get(ctx, "ps:/app/test/KEY", GetOptions{ForceRaw: true})
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	// ForceRaw should return the raw stored value regardless of tags
	if val != `"hello"` {
		t.Errorf("Get(ForceRaw) = %q, want %q", val, `"hello"`)
	}
}

func TestPSBackend_GetForceJSON(t *testing.T) {
	ctx := context.Background()

	client := &mockSSMClient{
		getParameterFn: func(_ context.Context, input *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
			return &ssm.GetParameterOutput{
				Parameter: &ssmtypes.Parameter{
					Name:  input.Name,
					Value: aws.String(`"hello"`),
				},
			}, nil
		},
		listTagsForResourceFn: func(_ context.Context, _ *ssm.ListTagsForResourceInput, _ ...func(*ssm.Options)) (*ssm.ListTagsForResourceOutput, error) {
			return &ssm.ListTagsForResourceOutput{
				TagList: []ssmtypes.Tag{
					{Key: aws.String(tags.TagStoreMode), Value: aws.String(tags.StoreModeRaw)},
				},
			}, nil
		},
	}

	backend := NewPSBackend(client)
	val, err := backend.Get(ctx, "ps:/app/test/KEY", GetOptions{ForceJSON: true})
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	// ForceJSON should decode even though tag says raw
	if val != "hello" {
		t.Errorf("Get(ForceJSON) = %q, want %q", val, "hello")
	}
}

// --- GetByPrefix tests (PS-GB-01 ~ PS-GB-07) ---

func rawTagList() []ssmtypes.Tag {
	return []ssmtypes.Tag{
		{Key: aws.String(tags.TagCLI), Value: aws.String(tags.TagCLIValue)},
		{Key: aws.String(tags.TagStoreMode), Value: aws.String(tags.StoreModeRaw)},
		{Key: aws.String(tags.TagSchema), Value: aws.String(tags.TagSchemaValue)},
	}
}

// PS-GB-01: Basic retrieval of 3 parameters with StoreModeRaw
func TestPSBackend_GetByPrefix_Basic(t *testing.T) {
	ctx := context.Background()

	client := &mockSSMClient{
		getParametersByPathFn: func(_ context.Context, input *ssm.GetParametersByPathInput, _ ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
			if aws.ToString(input.Path) != "/app/prod/" {
				t.Errorf("Path = %q, want %q", aws.ToString(input.Path), "/app/prod/")
			}
			if !aws.ToBool(input.WithDecryption) {
				t.Error("WithDecryption should be true")
			}
			if !aws.ToBool(input.Recursive) {
				t.Error("Recursive should be true")
			}
			return &ssm.GetParametersByPathOutput{
				Parameters: []ssmtypes.Parameter{
					{Name: aws.String("/app/prod/DB_HOST"), Value: aws.String("localhost")},
					{Name: aws.String("/app/prod/DB_PORT"), Value: aws.String("5432")},
					{Name: aws.String("/app/prod/DB_NAME"), Value: aws.String("mydb")},
				},
			}, nil
		},
		listTagsForResourceFn: func(_ context.Context, _ *ssm.ListTagsForResourceInput, _ ...func(*ssm.Options)) (*ssm.ListTagsForResourceOutput, error) {
			return &ssm.ListTagsForResourceOutput{TagList: rawTagList()}, nil
		},
	}

	backend := NewPSBackend(client)
	entries, err := backend.GetByPrefix(ctx, "/app/prod/", GetByPrefixOptions{Recursive: true})
	if err != nil {
		t.Fatalf("GetByPrefix() error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}
	for _, e := range entries {
		if e.StoreMode != tags.StoreModeRaw {
			t.Errorf("entry %s: StoreMode = %q, want %q", e.Path, e.StoreMode, tags.StoreModeRaw)
		}
	}
}

// PS-GB-02: Pagination (NextToken test)
func TestPSBackend_GetByPrefix_Paginated(t *testing.T) {
	ctx := context.Background()
	callCount := 0

	client := &mockSSMClient{
		getParametersByPathFn: func(_ context.Context, input *ssm.GetParametersByPathInput, _ ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
			callCount++
			if callCount == 1 {
				return &ssm.GetParametersByPathOutput{
					Parameters: []ssmtypes.Parameter{
						{Name: aws.String("/app/prod/KEY1"), Value: aws.String("v1")},
						{Name: aws.String("/app/prod/KEY2"), Value: aws.String("v2")},
					},
					NextToken: aws.String("tok"),
				}, nil
			}
			if aws.ToString(input.NextToken) != "tok" {
				t.Errorf("NextToken = %q, want %q", aws.ToString(input.NextToken), "tok")
			}
			return &ssm.GetParametersByPathOutput{
				Parameters: []ssmtypes.Parameter{
					{Name: aws.String("/app/prod/KEY3"), Value: aws.String("v3")},
					{Name: aws.String("/app/prod/KEY4"), Value: aws.String("v4")},
					{Name: aws.String("/app/prod/KEY5"), Value: aws.String("v5")},
				},
			}, nil
		},
		listTagsForResourceFn: func(_ context.Context, _ *ssm.ListTagsForResourceInput, _ ...func(*ssm.Options)) (*ssm.ListTagsForResourceOutput, error) {
			return &ssm.ListTagsForResourceOutput{TagList: rawTagList()}, nil
		},
	}

	backend := NewPSBackend(client)
	entries, err := backend.GetByPrefix(ctx, "/app/prod/", GetByPrefixOptions{Recursive: true})
	if err != nil {
		t.Fatalf("GetByPrefix() error: %v", err)
	}
	if len(entries) != 5 {
		t.Fatalf("got %d entries, want 5", len(entries))
	}
	if callCount != 2 {
		t.Errorf("GetParametersByPath called %d times, want 2", callCount)
	}
}

// PS-GB-03: Empty result
func TestPSBackend_GetByPrefix_EmptyResult(t *testing.T) {
	ctx := context.Background()

	client := &mockSSMClient{
		getParametersByPathFn: func(_ context.Context, _ *ssm.GetParametersByPathInput, _ ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
			return &ssm.GetParametersByPathOutput{
				Parameters: []ssmtypes.Parameter{},
			}, nil
		},
		listTagsForResourceFn: func(_ context.Context, _ *ssm.ListTagsForResourceInput, _ ...func(*ssm.Options)) (*ssm.ListTagsForResourceOutput, error) {
			return &ssm.ListTagsForResourceOutput{TagList: rawTagList()}, nil
		},
	}

	backend := NewPSBackend(client)
	entries, err := backend.GetByPrefix(ctx, "/app/prod/", GetByPrefixOptions{Recursive: true})
	if err != nil {
		t.Fatalf("GetByPrefix() error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("got %d entries, want 0", len(entries))
	}
}

// PS-GB-04: StoreModeFromTags (json tag)
func TestPSBackend_GetByPrefix_StoreModeFromTags(t *testing.T) {
	ctx := context.Background()

	client := &mockSSMClient{
		getParametersByPathFn: func(_ context.Context, _ *ssm.GetParametersByPathInput, _ ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
			return &ssm.GetParametersByPathOutput{
				Parameters: []ssmtypes.Parameter{
					{Name: aws.String("/app/prod/CONFIG"), Value: aws.String(`{"key":"value"}`)},
				},
			}, nil
		},
		listTagsForResourceFn: func(_ context.Context, _ *ssm.ListTagsForResourceInput, _ ...func(*ssm.Options)) (*ssm.ListTagsForResourceOutput, error) {
			return &ssm.ListTagsForResourceOutput{
				TagList: []ssmtypes.Tag{
					{Key: aws.String(tags.TagCLI), Value: aws.String(tags.TagCLIValue)},
					{Key: aws.String(tags.TagStoreMode), Value: aws.String(tags.StoreModeJSON)},
					{Key: aws.String(tags.TagSchema), Value: aws.String(tags.TagSchemaValue)},
				},
			}, nil
		},
	}

	backend := NewPSBackend(client)
	entries, err := backend.GetByPrefix(ctx, "/app/prod/", GetByPrefixOptions{Recursive: true})
	if err != nil {
		t.Fatalf("GetByPrefix() error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].StoreMode != tags.StoreModeJSON {
		t.Errorf("StoreMode = %q, want %q", entries[0].StoreMode, tags.StoreModeJSON)
	}
	if entries[0].Value != `{"key":"value"}` {
		t.Errorf("Value = %q, want %q", entries[0].Value, `{"key":"value"}`)
	}
}

// PS-GB-05: API error from GetParametersByPath
func TestPSBackend_GetByPrefix_APIError(t *testing.T) {
	ctx := context.Background()

	client := &mockSSMClient{
		getParametersByPathFn: func(_ context.Context, _ *ssm.GetParametersByPathInput, _ ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
			return nil, fmt.Errorf("AccessDeniedException")
		},
		listTagsForResourceFn: func(_ context.Context, _ *ssm.ListTagsForResourceInput, _ ...func(*ssm.Options)) (*ssm.ListTagsForResourceOutput, error) {
			return &ssm.ListTagsForResourceOutput{TagList: rawTagList()}, nil
		},
	}

	backend := NewPSBackend(client)
	_, err := backend.GetByPrefix(ctx, "/app/prod/", GetByPrefixOptions{Recursive: true})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "ssm GetParametersByPath") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "ssm GetParametersByPath")
	}
}

// PS-GB-06: ListTagsForResource error
func TestPSBackend_GetByPrefix_TagsAPIError(t *testing.T) {
	ctx := context.Background()

	client := &mockSSMClient{
		getParametersByPathFn: func(_ context.Context, _ *ssm.GetParametersByPathInput, _ ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
			return &ssm.GetParametersByPathOutput{
				Parameters: []ssmtypes.Parameter{
					{Name: aws.String("/app/prod/KEY"), Value: aws.String("val")},
				},
			}, nil
		},
		listTagsForResourceFn: func(_ context.Context, _ *ssm.ListTagsForResourceInput, _ ...func(*ssm.Options)) (*ssm.ListTagsForResourceOutput, error) {
			return nil, fmt.Errorf("InternalServerError")
		},
	}

	backend := NewPSBackend(client)
	_, err := backend.GetByPrefix(ctx, "/app/prod/", GetByPrefixOptions{Recursive: true})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "get store mode for") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "get store mode for")
	}
}

// --- Put 2-step tests (PutParameter + AddTagsToResource) ---

// TestPSBackend_Put_TwoStep verifies that Put calls PutParameter WITHOUT Tags,
// then calls AddTagsToResource separately with the managed tags.
func TestPSBackend_Put_TwoStep(t *testing.T) {
	ctx := context.Background()

	client := &mockSSMClient{
		putParameterFn: func(_ context.Context, _ *ssm.PutParameterInput, _ ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
			return &ssm.PutParameterOutput{}, nil
		},
		addTagsToResourceFn: func(_ context.Context, _ *ssm.AddTagsToResourceInput, _ ...func(*ssm.Options)) (*ssm.AddTagsToResourceOutput, error) {
			return &ssm.AddTagsToResourceOutput{}, nil
		},
	}

	backend := NewPSBackend(client)
	err := backend.Put(ctx, "ps:/app/test/KEY", PutOptions{
		Value:     "hello",
		StoreMode: tags.StoreModeRaw,
	})
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	// Step 1: PutParameter should be called WITHOUT Tags
	if len(client.putParameterCalls) != 1 {
		t.Fatalf("PutParameter called %d times, want 1", len(client.putParameterCalls))
	}
	if len(client.putParameterCalls[0].Tags) != 0 {
		t.Errorf("PutParameter should not have Tags, got %d tags", len(client.putParameterCalls[0].Tags))
	}

	// Step 2: AddTagsToResource should be called with managed tags
	if len(client.addTagsToResourceCalls) != 1 {
		t.Fatalf("AddTagsToResource called %d times, want 1", len(client.addTagsToResourceCalls))
	}
	addTagsInput := client.addTagsToResourceCalls[0]
	if aws.ToString(addTagsInput.ResourceId) != "/app/test/KEY" {
		t.Errorf("ResourceId = %q, want %q", aws.ToString(addTagsInput.ResourceId), "/app/test/KEY")
	}
	if addTagsInput.ResourceType != ssmtypes.ResourceTypeForTaggingParameter {
		t.Errorf("ResourceType = %v, want %v", addTagsInput.ResourceType, ssmtypes.ResourceTypeForTaggingParameter)
	}
	tagMap := make(map[string]string)
	for _, tag := range addTagsInput.Tags {
		tagMap[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
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

// TestPSBackend_Put_AddTagsFail_Error verifies that when AddTagsToResource fails,
// the error message clearly indicates the parameter was saved but tags are missing.
func TestPSBackend_Put_AddTagsFail_Error(t *testing.T) {
	ctx := context.Background()

	client := &mockSSMClient{
		putParameterFn: func(_ context.Context, _ *ssm.PutParameterInput, _ ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
			return &ssm.PutParameterOutput{}, nil
		},
		addTagsToResourceFn: func(_ context.Context, _ *ssm.AddTagsToResourceInput, _ ...func(*ssm.Options)) (*ssm.AddTagsToResourceOutput, error) {
			return nil, fmt.Errorf("AccessDeniedException: not authorized")
		},
	}

	backend := NewPSBackend(client)
	err := backend.Put(ctx, "ps:/app/test/KEY", PutOptions{
		Value:     "hello",
		StoreMode: tags.StoreModeRaw,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "AddTagsToResource failed") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "AddTagsToResource failed")
	}
	if !strings.Contains(err.Error(), "parameter was saved but tags are missing") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "parameter was saved but tags are missing")
	}
}

// PS-GB-07: No tags -> default raw
func TestPSBackend_GetByPrefix_DefaultStoreModeRaw(t *testing.T) {
	ctx := context.Background()

	client := &mockSSMClient{
		getParametersByPathFn: func(_ context.Context, _ *ssm.GetParametersByPathInput, _ ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
			return &ssm.GetParametersByPathOutput{
				Parameters: []ssmtypes.Parameter{
					{Name: aws.String("/app/prod/KEY"), Value: aws.String("val")},
				},
			}, nil
		},
		listTagsForResourceFn: func(_ context.Context, _ *ssm.ListTagsForResourceInput, _ ...func(*ssm.Options)) (*ssm.ListTagsForResourceOutput, error) {
			return &ssm.ListTagsForResourceOutput{
				TagList: []ssmtypes.Tag{}, // No tags
			}, nil
		},
	}

	backend := NewPSBackend(client)
	entries, err := backend.GetByPrefix(ctx, "/app/prod/", GetByPrefixOptions{Recursive: true})
	if err != nil {
		t.Fatalf("GetByPrefix() error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].StoreMode != tags.StoreModeRaw {
		t.Errorf("StoreMode = %q, want %q", entries[0].StoreMode, tags.StoreModeRaw)
	}
}
