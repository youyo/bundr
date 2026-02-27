package cmd

import (
	"fmt"
	"io"
	"os"
)

// CompletionCmd represents the "completion" subcommand.
type CompletionCmd struct {
	Shell string `arg:"" enum:"bash,zsh,fish" help:"Shell type (bash|zsh|fish)"`

	out io.Writer // for testing; nil means os.Stdout
}

// Run executes the completion command, writing the shell completion script to stdout.
// Usage: eval "$(bundr completion zsh)"
func (c *CompletionCmd) Run() error {
	if c.out == nil {
		c.out = os.Stdout
	}

	bin, err := os.Executable()
	if err != nil {
		bin = "bundr"
	}

	var script string
	switch c.Shell {
	case "bash":
		script = fmt.Sprintf("complete -C %s bundr\n", bin)
	case "zsh":
		script = fmt.Sprintf("autoload -U +X bashcompinit && bashcompinit\ncomplete -C %s bundr\n", bin)
	case "fish":
		script = fmt.Sprintf(
			"function __complete_bundr\n"+
				"  set -lx COMP_LINE (commandline -cp)\n"+
				"  test -z (commandline -cp)[-1]; and set COMP_LINE \"$COMP_LINE \"\n"+
				"  %s\n"+
				"end\n"+
				"complete -c bundr -a \"(__complete_bundr)\"\n",
			bin,
		)
	}

	_, err = fmt.Fprint(c.out, script)
	return err
}
