package dotenv

import (
	"bytes"
	"strings"
	"testing"
)

func TestParse_CommentsAndBlankLines(t *testing.T) {
	input := `# this is a comment
KEY1=value1

# another comment

KEY2=value2
`
	entries, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Key != "KEY1" || entries[0].Value != "value1" {
		t.Errorf("entry[0] = %+v, want {KEY1, value1}", entries[0])
	}
	if entries[1].Key != "KEY2" || entries[1].Value != "value2" {
		t.Errorf("entry[1] = %+v, want {KEY2, value2}", entries[1])
	}
}

func TestParse_SingleQuoted(t *testing.T) {
	input := "DB_HOST='localhost'\n"
	entries, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Value != "localhost" {
		t.Errorf("value = %q, want %q", entries[0].Value, "localhost")
	}
}

func TestParse_DoubleQuoted(t *testing.T) {
	input := `DB_HOST="localhost"` + "\n"
	entries, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Value != "localhost" {
		t.Errorf("value = %q, want %q", entries[0].Value, "localhost")
	}
}

func TestParse_Unquoted(t *testing.T) {
	input := "DB_HOST=localhost\n"
	entries, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Value != "localhost" {
		t.Errorf("value = %q, want %q", entries[0].Value, "localhost")
	}
}

func TestParse_ValueContainsEquals(t *testing.T) {
	input := "CONNECTION=host=localhost;port=5432\n"
	entries, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Key != "CONNECTION" {
		t.Errorf("key = %q, want %q", entries[0].Key, "CONNECTION")
	}
	if entries[0].Value != "host=localhost;port=5432" {
		t.Errorf("value = %q, want %q", entries[0].Value, "host=localhost;port=5432")
	}
}

func TestParse_EmptyValue(t *testing.T) {
	input := "EMPTY=\n"
	entries, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Value != "" {
		t.Errorf("value = %q, want empty", entries[0].Value)
	}
}

func TestParse_MissingEquals(t *testing.T) {
	input := "INVALID_LINE\n"
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for missing '=', got nil")
	}
}

func TestParse_EmptyKey(t *testing.T) {
	input := "=value\n"
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for empty key, got nil")
	}
}

func TestWrite_MultipleEntries(t *testing.T) {
	entries := []Entry{
		{Key: "DB_HOST", Value: "localhost"},
		{Key: "DB_PORT", Value: "5432"},
		{Key: "DB_NAME", Value: "mydb"},
	}
	var buf bytes.Buffer
	err := Write(&buf, entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "DB_HOST=localhost\nDB_PORT=5432\nDB_NAME=mydb\n"
	if buf.String() != expected {
		t.Errorf("output = %q, want %q", buf.String(), expected)
	}
}

func TestWrite_EmptyEntries(t *testing.T) {
	var buf bytes.Buffer
	err := Write(&buf, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.String() != "" {
		t.Errorf("output = %q, want empty", buf.String())
	}
}

func TestParse_AllFormats(t *testing.T) {
	input := `UNQUOTED=value1
SINGLE='value2'
DOUBLE="value3"
`
	entries, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	want := []Entry{
		{Key: "UNQUOTED", Value: "value1"},
		{Key: "SINGLE", Value: "value2"},
		{Key: "DOUBLE", Value: "value3"},
	}
	for i, e := range entries {
		if e.Key != want[i].Key || e.Value != want[i].Value {
			t.Errorf("entry[%d] = %+v, want %+v", i, e, want[i])
		}
	}
}

func TestRoundtrip(t *testing.T) {
	original := []Entry{
		{Key: "A", Value: "1"},
		{Key: "B", Value: "hello world"},
		{Key: "C", Value: "x=y"},
	}
	var buf bytes.Buffer
	if err := Write(&buf, original); err != nil {
		t.Fatalf("write: %v", err)
	}
	parsed, err := Parse(&buf)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(parsed) != len(original) {
		t.Fatalf("roundtrip: expected %d entries, got %d", len(original), len(parsed))
	}
	for i, e := range parsed {
		if e.Key != original[i].Key || e.Value != original[i].Value {
			t.Errorf("roundtrip[%d] = %+v, want %+v", i, e, original[i])
		}
	}
}
