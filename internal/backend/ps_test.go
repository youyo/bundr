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
	describeParametersFn    func(ctx context.Context, input *ssm.DescribeParametersInput, optFns ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error)

	// Call recording fields for verifying call sequences
	putParameterCalls      []*ssm.PutParameterInput
	addTagsToResourceCalls []*ssm.AddTagsToResourceInput
}

func (m *mockSSMClient) PutParameter(ctx context.Context, input *ssm.PutParameterInput, optFns ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
	m.putParameterCalls = append(m.putParameterCalls, input)
	return m.putParameterFn(ctx, input, optFns...)
}

func (m *mockSSMClient) GetParameter(ctx context.Context, input *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	if m.getParameterFn == nil {
		return nil, fmt.Errorf("ParameterNotFound")
	}
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

func (m *mockSSMClient) DescribeParameters(ctx context.Context, input *ssm.DescribeParametersInput, optFns ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error) {
	if m.describeParametersFn == nil {
		// Default: parameter not found (new parameter → Standard tier)
		return &ssm.DescribeParametersOutput{Parameters: nil}, nil
	}
	return m.describeParametersFn(ctx, input, optFns...)
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

// TestPSBackend_PutAdvancedTier_PSAPrefix verifies that psa: prefix still sets Advanced tier
// (backward compat: psa: normalizes to BackendTypePS + AdvancedTier=true).
func TestPSBackend_PutAdvancedTier_PSAPrefix(t *testing.T) {
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

// TestPSBackend_PutAdvancedTier_OptsFlag verifies that PutOptions.AdvancedTier=true sets Advanced tier.
func TestPSBackend_PutAdvancedTier_OptsFlag(t *testing.T) {
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
		Value:        "hello",
		StoreMode:    tags.StoreModeRaw,
		AdvancedTier: true,
		TierExplicit: true,
	})
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	if capturedInput.Tier != ssmtypes.ParameterTierAdvanced {
		t.Errorf("Tier = %v, want %v", capturedInput.Tier, ssmtypes.ParameterTierAdvanced)
	}
}

// TestPSBackend_PutAutoDetect_ExistingAdvanced verifies that an existing Advanced param
// is kept Advanced when ps: is used without explicit --tier.
func TestPSBackend_PutAutoDetect_ExistingAdvanced(t *testing.T) {
	ctx := context.Background()
	var capturedInput *ssm.PutParameterInput

	client := &mockSSMClient{
		describeParametersFn: func(_ context.Context, _ *ssm.DescribeParametersInput, _ ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error) {
			return &ssm.DescribeParametersOutput{
				Parameters: []ssmtypes.ParameterMetadata{
					{
						Name: aws.String("/app/test/KEY"),
						Tier: ssmtypes.ParameterTierAdvanced,
					},
				},
			}, nil
		},
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
		Value:     "new-value",
		StoreMode: tags.StoreModeRaw,
	})
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	// auto-detect should preserve Advanced tier
	if capturedInput.Tier != ssmtypes.ParameterTierAdvanced {
		t.Errorf("Tier = %v, want %v (auto-detect should keep Advanced)", capturedInput.Tier, ssmtypes.ParameterTierAdvanced)
	}
}

// TestPSBackend_PutAutoDetect_NewParam verifies that a new parameter defaults to Standard tier.
func TestPSBackend_PutAutoDetect_NewParam(t *testing.T) {
	ctx := context.Background()
	var capturedInput *ssm.PutParameterInput

	client := &mockSSMClient{
		// getParameterFn returns nil (ParameterNotFound) by default
		putParameterFn: func(_ context.Context, input *ssm.PutParameterInput, _ ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
			capturedInput = input
			return &ssm.PutParameterOutput{}, nil
		},
		addTagsToResourceFn: func(_ context.Context, _ *ssm.AddTagsToResourceInput, _ ...func(*ssm.Options)) (*ssm.AddTagsToResourceOutput, error) {
			return &ssm.AddTagsToResourceOutput{}, nil
		},
	}

	backend := NewPSBackend(client)
	err := backend.Put(ctx, "ps:/app/test/NEWKEY", PutOptions{
		Value:     "hello",
		StoreMode: tags.StoreModeRaw,
	})
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	// new param → Standard (no tier specified in input means Standard)
	if capturedInput.Tier == ssmtypes.ParameterTierAdvanced {
		t.Errorf("Tier = %v, want Standard for new param", capturedInput.Tier)
	}
}

// TestPSBackend_PutTierExplicitStandard verifies that --tier standard skips auto-detect
// even when existing param is Advanced.
func TestPSBackend_PutTierExplicitStandard(t *testing.T) {
	ctx := context.Background()
	describeParametersCalled := 0
	var capturedInput *ssm.PutParameterInput

	client := &mockSSMClient{
		describeParametersFn: func(_ context.Context, _ *ssm.DescribeParametersInput, _ ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error) {
			describeParametersCalled++
			return &ssm.DescribeParametersOutput{
				Parameters: []ssmtypes.ParameterMetadata{
					{Tier: ssmtypes.ParameterTierAdvanced},
				},
			}, nil
		},
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
		Value:        "hello",
		StoreMode:    tags.StoreModeRaw,
		AdvancedTier: false,
		TierExplicit: true, // --tier standard
	})
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	// TierExplicit=true skips auto-detect → no DescribeParameters call
	if describeParametersCalled != 0 {
		t.Errorf("DescribeParameters called %d times, want 0 (TierExplicit skips auto-detect)", describeParametersCalled)
	}
	// Standard tier (zero value, not Advanced)
	if capturedInput.Tier == ssmtypes.ParameterTierAdvanced {
		t.Errorf("Tier = %v, want Standard when --tier standard is explicit", capturedInput.Tier)
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

// PS-GB-08: SkipTagFetch=true → ListTagsForResource が呼ばれない、StoreMode は空文字
func TestPSBackend_GetByPrefix_SkipTagFetch(t *testing.T) {
	ctx := context.Background()
	listTagsCalled := 0

	client := &mockSSMClient{
		getParametersByPathFn: func(_ context.Context, _ *ssm.GetParametersByPathInput, _ ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
			return &ssm.GetParametersByPathOutput{
				Parameters: []ssmtypes.Parameter{
					{Name: aws.String("/app/prod/KEY1"), Value: aws.String("v1")},
					{Name: aws.String("/app/prod/KEY2"), Value: aws.String("v2")},
				},
			}, nil
		},
		listTagsForResourceFn: func(_ context.Context, _ *ssm.ListTagsForResourceInput, _ ...func(*ssm.Options)) (*ssm.ListTagsForResourceOutput, error) {
			listTagsCalled++
			return &ssm.ListTagsForResourceOutput{TagList: rawTagList()}, nil
		},
	}

	b := NewPSBackend(client)
	entries, err := b.GetByPrefix(ctx, "/app/prod/", GetByPrefixOptions{Recursive: true, SkipTagFetch: true})
	if err != nil {
		t.Fatalf("GetByPrefix() error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if listTagsCalled != 0 {
		t.Errorf("ListTagsForResource called %d times, want 0 (SkipTagFetch=true)", listTagsCalled)
	}
	for _, e := range entries {
		if e.StoreMode != "" {
			t.Errorf("entry %s: StoreMode = %q, want empty string (SkipTagFetch=true)", e.Path, e.StoreMode)
		}
	}
}

// PS-D-01: Describe (SecureString) - returns Name, Type, Value, Version, ARN, DataType, LastModifiedDate
func TestPSBackendDescribe_SecureString(t *testing.T) {
	ctx := context.Background()

	client := &mockSSMClient{
		getParameterFn: func(_ context.Context, input *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
			return &ssm.GetParameterOutput{
				Parameter: &ssmtypes.Parameter{
					Name:    aws.String("/app/db_host"),
					Type:    ssmtypes.ParameterTypeSecureString,
					Value:   aws.String("localhost"),
					Version: 3,
					ARN:     aws.String("arn:aws:ssm:ap-northeast-1:123456789:parameter/app/db_host"),
					DataType: aws.String("text"),
				},
			}, nil
		},
	}

	b := NewPSBackend(client)
	result, err := b.Describe(ctx, "ps:/app/db_host")
	if err != nil {
		t.Fatalf("Describe() error: %v", err)
	}

	if result["Name"] != "/app/db_host" {
		t.Errorf("Name = %v, want %q", result["Name"], "/app/db_host")
	}
	if result["Type"] != "SecureString" {
		t.Errorf("Type = %v, want %q", result["Type"], "SecureString")
	}
	if result["Value"] != "localhost" {
		t.Errorf("Value = %v, want %q", result["Value"], "localhost")
	}
	if result["Version"] != int64(3) {
		t.Errorf("Version = %v, want %v", result["Version"], int64(3))
	}
	if result["ARN"] != "arn:aws:ssm:ap-northeast-1:123456789:parameter/app/db_host" {
		t.Errorf("ARN = %v, want expected ARN", result["ARN"])
	}
	if result["DataType"] != "text" {
		t.Errorf("DataType = %v, want %q", result["DataType"], "text")
	}
	// LastModifiedDate should be present (nil is OK for mock)
	if _, ok := result["LastModifiedDate"]; !ok {
		t.Error("LastModifiedDate key missing from result")
	}
}

// PS-D-ERR-01: Describe with nonexistent ref returns error
func TestPSBackendDescribe_NotFound(t *testing.T) {
	ctx := context.Background()

	client := &mockSSMClient{
		getParameterFn: func(_ context.Context, _ *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
			return nil, fmt.Errorf("ParameterNotFound")
		},
	}

	b := NewPSBackend(client)
	_, err := b.Describe(ctx, "ps:/nonexistent")
	if err == nil {
		t.Fatal("Describe() expected error for nonexistent ref, got nil")
	}
	if !strings.Contains(err.Error(), "ssm GetParameter") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "ssm GetParameter")
	}
}

// PS-GB-D-01: GetByPrefix with IncludeMetadata=true populates Metadata
func TestPSBackend_GetByPrefix_IncludeMetadata(t *testing.T) {
	ctx := context.Background()

	client := &mockSSMClient{
		getParametersByPathFn: func(_ context.Context, _ *ssm.GetParametersByPathInput, _ ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
			return &ssm.GetParametersByPathOutput{
				Parameters: []ssmtypes.Parameter{
					{
						Name:     aws.String("/app/db_host"),
						Type:     ssmtypes.ParameterTypeSecureString,
						Value:    aws.String("localhost"),
						Version:  3,
						ARN:      aws.String("arn:aws:ssm:region:123:parameter/app/db_host"),
						DataType: aws.String("text"),
					},
					{
						Name:     aws.String("/app/db_port"),
						Type:     ssmtypes.ParameterTypeString,
						Value:    aws.String("5432"),
						Version:  1,
						ARN:      aws.String("arn:aws:ssm:region:123:parameter/app/db_port"),
						DataType: aws.String("text"),
					},
				},
			}, nil
		},
		listTagsForResourceFn: func(_ context.Context, _ *ssm.ListTagsForResourceInput, _ ...func(*ssm.Options)) (*ssm.ListTagsForResourceOutput, error) {
			return &ssm.ListTagsForResourceOutput{TagList: rawTagList()}, nil
		},
	}

	b := NewPSBackend(client)
	entries, err := b.GetByPrefix(ctx, "/app/", GetByPrefixOptions{
		Recursive:       true,
		IncludeMetadata: true,
	})
	if err != nil {
		t.Fatalf("GetByPrefix() error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	for _, e := range entries {
		if e.Metadata == nil {
			t.Errorf("entry %s: Metadata is nil, want non-nil", e.Path)
			continue
		}
		if _, ok := e.Metadata["ARN"]; !ok {
			t.Errorf("entry %s: Metadata missing ARN key", e.Path)
		}
		if _, ok := e.Metadata["Version"]; !ok {
			t.Errorf("entry %s: Metadata missing Version key", e.Path)
		}
		if _, ok := e.Metadata["Type"]; !ok {
			t.Errorf("entry %s: Metadata missing Type key", e.Path)
		}
		// Metadata should NOT contain Value
		if _, ok := e.Metadata["Value"]; ok {
			t.Errorf("entry %s: Metadata should not contain Value", e.Path)
		}
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
