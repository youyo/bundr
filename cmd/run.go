package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// SubprocessRunner abstracts subprocess execution for testability.
type SubprocessRunner interface {
	Run(name string, args []string, env []string) (int, error)
}

// OsExecRunner is the production implementation of SubprocessRunner.
// I/O is connected directly to os.Stdin/Stdout/Stderr.
type OsExecRunner struct{}

// Run executes the named program with args and env using os/exec.
func (r *OsExecRunner) Run(name string, args []string, env []string) (int, error) {
	c := exec.Command(name, args...)
	c.Env = env
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), err
		}
		return 1, err
	}
	return 0, nil
}

// ExitCodeError carries the exit code of a child process.
// main.go uses errors.As to convert this to os.Exit.
type ExitCodeError struct {
	Code int
}

func (e *ExitCodeError) Error() string {
	return fmt.Sprintf("exit status %d", e.Code)
}

// RunCmd represents the "run" subcommand.
type RunCmd struct {
	From           []string `short:"f" name:"from" optional:"" predictor:"prefix" help:"Source prefixes (e.g. ps:/app/prod/); later entries take precedence"`
	NoFlatten      bool     `name:"no-flatten" help:"Disable JSON flattening"`
	ArrayMode      string   `default:"join" enum:"join,index,json" help:"Array handling mode"`
	ArrayJoinDelim string   `default:"," help:"Delimiter for array join mode"`
	FlattenDelim   string   `default:"_" help:"Delimiter for flattened keys"`
	Upper          bool     `default:"true" negatable:"" help:"Uppercase variable names"`
	Args           []string `arg:"" optional:"" passthrough:"" help:"Command and arguments to run"`

	runner SubprocessRunner // nil means OsExecRunner (injected for testing)
}

// Run executes the run command.
func (c *RunCmd) Run(appCtx *Context) error {
	if len(c.Args) == 0 {
		return fmt.Errorf("run command failed: no command specified")
	}

	runner := c.runner
	if runner == nil {
		runner = &OsExecRunner{}
	}

	// Process multiple From prefixes in order; later entries take precedence.
	vars := make(map[string]string)
	for _, from := range c.From {
		result, err := buildVars(context.Background(), appCtx, VarsBuildOptions{
			From:           from,
			FlattenDelim:   c.FlattenDelim,
			ArrayMode:      c.ArrayMode,
			ArrayJoinDelim: c.ArrayJoinDelim,
			Upper:          c.Upper,
			NoFlatten:      c.NoFlatten,
		})
		if err != nil {
			return fmt.Errorf("run command failed: %w", err)
		}
		for k, v := range result {
			vars[k] = v
		}
	}

	// Start from current environment and append fetched vars.
	env := os.Environ()
	for k, v := range vars {
		env = append(env, k+"="+v)
	}

	exitCode, err := runner.Run(c.Args[0], c.Args[1:], env)
	if err != nil {
		if exitCode != 0 {
			return &ExitCodeError{Code: exitCode}
		}
		return fmt.Errorf("run command failed: %w", err)
	}
	return nil
}
