package backend

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/youyo/bundr/internal/tags"
)

// mockSSMClient is a test double for the SSM API.
type mockSSMClient struct {
	putParameterFn        func(ctx context.Context, input *ssm.PutParameterInput, optFns ...func(*ssm.Options)) (*ssm.PutParameterOutput, error)
	getParameterFn        func(ctx context.Context, input *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
	addTagsToResourceFn   func(ctx context.Context, input *ssm.AddTagsToResourceInput, optFns ...func(*ssm.Options)) (*ssm.AddTagsToResourceOutput, error)
	listTagsForResourceFn func(ctx context.Context, input *ssm.ListTagsForResourceInput, optFns ...func(*ssm.Options)) (*ssm.ListTagsForResourceOutput, error)
}

func (m *mockSSMClient) PutParameter(ctx context.Context, input *ssm.PutParameterInput, optFns ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
	return m.putParameterFn(ctx, input, optFns...)
}

func (m *mockSSMClient) GetParameter(ctx context.Context, input *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	return m.getParameterFn(ctx, input, optFns...)
}

func (m *mockSSMClient) AddTagsToResource(ctx context.Context, input *ssm.AddTagsToResourceInput, optFns ...func(*ssm.Options)) (*ssm.AddTagsToResourceOutput, error) {
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
		putParameterFn: func(_ context.Context, input *ssm.PutParameterInput, _ ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
			capturedTags = input.Tags
			return &ssm.PutParameterOutput{}, nil
		},
		addTagsToResourceFn: func(_ context.Context, input *ssm.AddTagsToResourceInput, _ ...func(*ssm.Options)) (*ssm.AddTagsToResourceOutput, error) {
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

	// Should have the 3 managed tags
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
