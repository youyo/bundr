package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestCompletionCmd(t *testing.T) {
	tests := []struct {
		id    string
		shell string
		want  string // 出力に含まれるべき文字列
	}{
		{
			// C-01: bash → "_bundr_complete" ラッパー関数と "COMP_POINT=${#COMP_LINE}" を含む補完スクリプト
			id:    "C-01",
			shell: "bash",
			want:  "_bundr_complete",
		},
		{
			// C-02: zsh → "bashcompinit" と "_bundr_complete" ラッパー関数を含む補完スクリプト
			id:    "C-02",
			shell: "zsh",
			want:  "_bundr_complete",
		},
		{
			// C-03: fish → "__complete_bundr" を含む補完スクリプト
			id:    "C-03",
			shell: "fish",
			want:  "__complete_bundr",
		},
	}

	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			var buf bytes.Buffer
			cmd := &CompletionCmd{
				Shell: tc.shell,
				out:   &buf,
			}

			err := cmd.Run()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			output := buf.String()
			if !strings.Contains(output, tc.want) {
				t.Errorf("output %q does not contain %q", output, tc.want)
			}
		})
	}
}

func TestCompletionCmd_ContainsBundr(t *testing.T) {
	// 全シェルで "bundr" がスクリプトに含まれることを確認
	for _, shell := range []string{"bash", "zsh", "fish"} {
		t.Run(shell, func(t *testing.T) {
			var buf bytes.Buffer
			cmd := &CompletionCmd{
				Shell: shell,
				out:   &buf,
			}
			if err := cmd.Run(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(buf.String(), "bundr") {
				t.Errorf("%s script does not contain 'bundr'", shell)
			}
		})
	}
}
