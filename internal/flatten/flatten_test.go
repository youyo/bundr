package flatten_test

import (
	"testing"

	"github.com/youyo/bundr/internal/flatten"
)

func TestFlatten(t *testing.T) {
	defaultOpts := flatten.DefaultOptions()

	tests := []struct {
		id      string
		prefix  string
		raw     string
		opts    flatten.Options
		want    map[string]string
		wantErr bool
	}{
		// --- Normal cases ---
		{
			id:     "F-01",
			prefix: "DB_HOST",
			raw:    "localhost",
			opts:   defaultOpts,
			want:   map[string]string{"DB_HOST": "localhost"},
		},
		{
			id:     "F-02",
			prefix: "DB_HOST",
			raw:    `"localhost"`,
			opts:   defaultOpts,
			want:   map[string]string{"DB_HOST": "localhost"},
		},
		{
			id:     "F-03",
			prefix: "DB",
			raw:    `{"host":"localhost","port":5432}`,
			opts:   defaultOpts,
			want:   map[string]string{"DB_HOST": "localhost", "DB_PORT": "5432"},
		},
		{
			id:     "F-04",
			prefix: "DB",
			raw:    `{"host":"localhost","port":5432}`,
			opts: flatten.Options{
				Delimiter:     "_",
				ArrayMode:     "join",
				ArrayJoinDelim: ",",
				Upper:         false,
				NoFlatten:     false,
			},
			want: map[string]string{"db_host": "localhost", "db_port": "5432"},
		},
		{
			id:     "F-05",
			prefix: "DB",
			raw:    `{"conn":{"host":"localhost","port":5432}}`,
			opts:   defaultOpts,
			want:   map[string]string{"DB_CONN_HOST": "localhost", "DB_CONN_PORT": "5432"},
		},
		{
			id:     "F-06",
			prefix: "TAGS",
			raw:    `["a","b","c"]`,
			opts: flatten.Options{
				Delimiter:     "_",
				ArrayMode:     "join",
				ArrayJoinDelim: ",",
				Upper:         true,
				NoFlatten:     false,
			},
			want: map[string]string{"TAGS": "a,b,c"},
		},
		{
			id:     "F-07",
			prefix: "TAGS",
			raw:    `["a","b","c"]`,
			opts: flatten.Options{
				Delimiter:     "_",
				ArrayMode:     "join",
				ArrayJoinDelim: ":",
				Upper:         true,
				NoFlatten:     false,
			},
			want: map[string]string{"TAGS": "a:b:c"},
		},
		{
			id:     "F-08",
			prefix: "TAGS",
			raw:    `["a","b","c"]`,
			opts: flatten.Options{
				Delimiter:     "_",
				ArrayMode:     "index",
				ArrayJoinDelim: ",",
				Upper:         true,
				NoFlatten:     false,
			},
			want: map[string]string{"TAGS_0": "a", "TAGS_1": "b", "TAGS_2": "c"},
		},
		{
			id:     "F-09",
			prefix: "TAGS",
			raw:    `["a","b","c"]`,
			opts: flatten.Options{
				Delimiter:     "_",
				ArrayMode:     "json",
				ArrayJoinDelim: ",",
				Upper:         true,
				NoFlatten:     false,
			},
			want: map[string]string{"TAGS": `["a","b","c"]`},
		},
		{
			id:     "F-10",
			prefix: "SERVERS",
			raw:    `[{"host":"x"},{"host":"y"}]`,
			opts: flatten.Options{
				Delimiter:     "_",
				ArrayMode:     "join",
				ArrayJoinDelim: ",",
				Upper:         true,
				NoFlatten:     false,
			},
			want: map[string]string{"SERVERS_0_HOST": "x", "SERVERS_1_HOST": "y"},
		},
		{
			id:     "F-11",
			prefix: "SERVERS",
			raw:    `[{"host":"x"},{"host":"y"}]`,
			opts: flatten.Options{
				Delimiter:     "_",
				ArrayMode:     "index",
				ArrayJoinDelim: ",",
				Upper:         true,
				NoFlatten:     false,
			},
			want: map[string]string{"SERVERS_0_HOST": "x", "SERVERS_1_HOST": "y"},
		},
		{
			id:     "F-12",
			prefix: "MIXED",
			raw:    `["a",{"key":"v"}]`,
			opts: flatten.Options{
				Delimiter:     "_",
				ArrayMode:     "join",
				ArrayJoinDelim: ",",
				Upper:         true,
				NoFlatten:     false,
			},
			want: map[string]string{"MIXED_0": "a", "MIXED_1_KEY": "v"},
		},
		{
			id:     "F-13",
			prefix: "NUM",
			raw:    `42`,
			opts:   defaultOpts,
			want:   map[string]string{"NUM": "42"},
		},
		{
			id:     "F-14",
			prefix: "FLAG",
			raw:    `true`,
			opts:   defaultOpts,
			want:   map[string]string{"FLAG": "true"},
		},
		{
			id:     "F-15",
			prefix: "NIL",
			raw:    `null`,
			opts:   defaultOpts,
			want:   map[string]string{"NIL": ""},
		},
		{
			id:     "F-16",
			prefix: "DB_HOST",
			raw:    "localhost",
			opts: flatten.Options{
				Delimiter:     "_",
				ArrayMode:     "join",
				ArrayJoinDelim: ",",
				Upper:         true,
				NoFlatten:     true,
			},
			want: map[string]string{"DB_HOST": "localhost"},
		},
		{
			id:     "F-17",
			prefix: "DB",
			raw:    `{"host":"x"}`,
			opts: flatten.Options{
				Delimiter:     "_",
				ArrayMode:     "join",
				ArrayJoinDelim: ",",
				Upper:         true,
				NoFlatten:     true,
			},
			want: map[string]string{"DB": `{"host":"x"}`},
		},
		{
			id:     "F-18",
			prefix: "STRS",
			raw:    `["[\"nested\"]","b"]`,
			opts: flatten.Options{
				Delimiter:     "_",
				ArrayMode:     "join",
				ArrayJoinDelim: ",",
				Upper:         true,
				NoFlatten:     false,
			},
			// Element 0 is a JSON-valid string -> join fails -> index fallback.
			// Element 0 re-parsed as ["nested"] array -> join succeeds -> STRS_0 = "nested".
			want: map[string]string{"STRS_0": "nested", "STRS_1": "b"},
		},
		{
			id:     "F-19",
			prefix: "DEEP",
			raw:    `{"a":{"b":{"c":"val"}}}`,
			opts:   defaultOpts,
			want:   map[string]string{"DEEP_A_B_C": "val"},
		},
		{
			id:     "F-20",
			prefix: "K",
			raw:    `{"key-name":"v"}`,
			opts:   defaultOpts,
			want:   map[string]string{"K_KEY_NAME": "v"},
		},
		{
			id:     "F-21",
			prefix: "K",
			raw:    `{"key_name":"v"}`,
			opts: flatten.Options{
				Delimiter:     "_",
				ArrayMode:     "join",
				ArrayJoinDelim: ",",
				Upper:         false,
				NoFlatten:     false,
			},
			want: map[string]string{"k_key_name": "v"},
		},
		{
			id:     "F-22",
			prefix: "PRICE",
			raw:    `42.5`,
			opts:   defaultOpts,
			want:   map[string]string{"PRICE": "42.5"},
		},
		{
			id:     "F-23",
			prefix: "COUNT",
			raw:    `100`,
			opts:   defaultOpts,
			want:   map[string]string{"COUNT": "100"},
		},

		// --- Error cases (treated as raw string, not actual errors) ---
		{
			id:     "FE-01",
			prefix: "KEY",
			raw:    "{invalid json",
			opts:   defaultOpts,
			want:   map[string]string{"KEY": "{invalid json"},
		},
		{
			id:     "FE-02",
			prefix: "KEY",
			raw:    "",
			opts:   defaultOpts,
			want:   map[string]string{"KEY": ""},
		},

		// --- Edge cases ---
		{
			id:     "FEC-01",
			prefix: "A",
			raw:    `[]`,
			opts:   defaultOpts,
			want:   map[string]string{},
		},
		{
			id:     "FEC-02",
			prefix: "",
			raw:    `{"k":"v"}`,
			opts:   defaultOpts,
			want:   map[string]string{"K": "v"},
		},
		{
			id:     "FEC-03",
			prefix: "ROOT",
			raw:    `{"a":{"b":{"c":{"d":{"e":{"f":{"g":{"h":{"i":{"j":"v"}}}}}}}}}}`,
			opts:   defaultOpts,
			want:   map[string]string{"ROOT_A_B_C_D_E_F_G_H_I_J": "v"},
		},
		{
			id:     "FEC-04",
			prefix: "A",
			raw:    `{}`,
			opts:   defaultOpts,
			want:   map[string]string{},
		},
		{
			id:     "FEC-05",
			prefix: "A",
			raw:    `["x",null,"y"]`,
			opts: flatten.Options{
				Delimiter:     "_",
				ArrayMode:     "join",
				ArrayJoinDelim: ",",
				Upper:         true,
				NoFlatten:     false,
			},
			want: map[string]string{"A_0": "x", "A_1": "", "A_2": "y"},
		},
		{
			id:     "FEC-06",
			prefix: "DB",
			raw:    `{"host":"localhost"}`,
			opts: flatten.Options{
				Delimiter:     "__",
				ArrayMode:     "join",
				ArrayJoinDelim: ",",
				Upper:         true,
				NoFlatten:     false,
			},
			want: map[string]string{"DB__HOST": "localhost"},
		},
		{
			id:     "FEC-07",
			prefix: "P",
			raw:    `{"1":"v"}`,
			opts:   defaultOpts,
			want:   map[string]string{"P_1": "v"},
		},
		{
			id:     "FEC-08",
			prefix: "A",
			raw:    `["only"]`,
			opts: flatten.Options{
				Delimiter:     "_",
				ArrayMode:     "join",
				ArrayJoinDelim: ",",
				Upper:         true,
				NoFlatten:     false,
			},
			want: map[string]string{"A": "only"},
		},
		{
			id:     "FEC-09",
			prefix: "A",
			raw:    `[true,false]`,
			opts: flatten.Options{
				Delimiter:     "_",
				ArrayMode:     "join",
				ArrayJoinDelim: ",",
				Upper:         true,
				NoFlatten:     false,
			},
			want: map[string]string{"A_0": "true", "A_1": "false"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			got, err := flatten.Flatten(tc.prefix, tc.raw, tc.opts)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tc.want) {
				t.Fatalf("length mismatch: got %d entries %v, want %d entries %v", len(got), got, len(tc.want), tc.want)
			}
			for k, wantV := range tc.want {
				gotV, ok := got[k]
				if !ok {
					t.Errorf("missing key %q; got %v", k, got)
					continue
				}
				if gotV != wantV {
					t.Errorf("key %q: got %q, want %q", k, gotV, wantV)
				}
			}
		})
	}
}

func TestApplyCasing(t *testing.T) {
	tests := []struct {
		name string
		key  string
		opts flatten.Options
		want string
	}{
		{
			name: "upper with hyphen",
			key:  "key-name",
			opts: flatten.Options{Upper: true},
			want: "KEY_NAME",
		},
		{
			name: "lower with hyphen",
			key:  "KEY-NAME",
			opts: flatten.Options{Upper: false},
			want: "key_name",
		},
		{
			name: "upper with underscore",
			key:  "key_name",
			opts: flatten.Options{Upper: true},
			want: "KEY_NAME",
		},
		{
			name: "lower with underscore",
			key:  "KEY_NAME",
			opts: flatten.Options{Upper: false},
			want: "key_name",
		},
		{
			name: "empty string upper",
			key:  "",
			opts: flatten.Options{Upper: true},
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := flatten.ApplyCasing(tc.key, tc.opts)
			if got != tc.want {
				t.Errorf("ApplyCasing(%q): got %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}
