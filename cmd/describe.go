package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/youyo/bundr/internal/backend"
)

// DescribeCmd represents the "describe" subcommand.
type DescribeCmd struct {
	Ref  string `arg:"" predictor:"ref" help:"Target ref (e.g. ps:/app/prod/DB_HOST, sm:secret-id)"`
	JSON bool   `name:"json" help:"Output in JSON format"`
}

// Run executes the describe command.
func (c *DescribeCmd) Run(appCtx *Context) error {
	ref, err := backend.ParseRef(c.Ref)
	if err != nil {
		return fmt.Errorf("describe command failed: invalid ref: %w", err)
	}

	b, err := appCtx.BackendFactory(ref.Type)
	if err != nil {
		return fmt.Errorf("describe command failed: create backend: %w", err)
	}

	out, err := b.Describe(context.Background(), c.Ref)
	if err != nil {
		return fmt.Errorf("describe command failed: %w", err)
	}

	if c.JSON {
		return c.printJSON(out, ref)
	}

	c.printText(out, ref)
	return nil
}

func (c *DescribeCmd) printJSON(out *backend.DescribeOutput, ref backend.Ref) error {
	m := map[string]any{
		"ref":  c.Ref,
		"path": out.Path,
		"tags": out.Tags,
	}
	if out.ARN != "" {
		m["arn"] = out.ARN
	}
	if out.Version != 0 {
		m["version"] = out.Version
	}
	if out.LastModifiedDate != nil {
		m["lastModifiedDate"] = out.LastModifiedDate
	}

	switch ref.Type {
	case backend.BackendTypePS, backend.BackendTypePSA:
		m["backend"] = backendLabel(ref.Type)
		if out.ParameterType != "" {
			m["parameterType"] = out.ParameterType
		}
		if out.Tier != "" {
			m["tier"] = out.Tier
		}
		if out.DataType != "" {
			m["dataType"] = out.DataType
		}
	case backend.BackendTypeSM:
		m["backend"] = backendLabel(ref.Type)
		if out.CreatedDate != nil {
			m["createdDate"] = out.CreatedDate
		}
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("json encode: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func (c *DescribeCmd) printText(out *backend.DescribeOutput, ref backend.Ref) {
	fmt.Printf("%-16s %s\n", "Ref:", c.Ref)
	fmt.Printf("%-16s %s\n", "Path:", out.Path)
	fmt.Printf("%-16s %s\n", "Backend:", backendLabel(ref.Type))

	if out.ARN != "" {
		fmt.Printf("%-16s %s\n", "ARN:", out.ARN)
	}

	switch ref.Type {
	case backend.BackendTypePS, backend.BackendTypePSA:
		if out.ParameterType != "" {
			fmt.Printf("%-16s %s\n", "Type:", out.ParameterType)
		}
		if out.Tier != "" {
			fmt.Printf("%-16s %s\n", "Tier:", out.Tier)
		}
		if out.DataType != "" {
			fmt.Printf("%-16s %s\n", "DataType:", out.DataType)
		}
	case backend.BackendTypeSM:
		if out.CreatedDate != nil {
			fmt.Printf("%-16s %s\n", "CreatedDate:", out.CreatedDate.UTC().Format("2006-01-02T15:04:05Z"))
		}
	}

	if out.Version != 0 {
		fmt.Printf("%-16s %d\n", "Version:", out.Version)
	}
	if out.LastModifiedDate != nil {
		fmt.Printf("%-16s %s\n", "LastModified:", out.LastModifiedDate.UTC().Format("2006-01-02T15:04:05Z"))
	}

	if len(out.Tags) > 0 {
		fmt.Println("Tags:")
		keys := make([]string, 0, len(out.Tags))
		for k := range out.Tags {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("  %-20s %s\n", k, out.Tags[k])
		}
	}
}

func backendLabel(bt backend.BackendType) string {
	switch bt {
	case backend.BackendTypePS:
		return "Parameter Store (Standard)"
	case backend.BackendTypePSA:
		return "Parameter Store (Advanced)"
	case backend.BackendTypeSM:
		return "Secrets Manager"
	default:
		return string(bt)
	}
}
