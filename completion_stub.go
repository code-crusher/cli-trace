//go:build urfave_cli_no_completion

package cli

import (
	"context"
	"errors"
	"fmt"
)

// completionSupported is true unless built with the urfave_cli_no_completion
// build tag. The tag removes the hidden "completion" command, the embedded
// shell scripts and the completion machinery, so that applications that do
// not enable shell completion do not pay for it in binary size (see #2291).
const completionSupported = false

var errNoCompletion = errors.New("shell completion is not supported when built with urfave_cli_no_completion tag")

// setupShellCompletion is a no-op: with the urfave_cli_no_completion tag
// there is no completion machinery to default to. Setting a custom
// completion function is a programming error and panics.
func (cmd *Command) setupShellCompletion() {
	if cmd.ShellComplete != nil {
		panic(fmt.Errorf("command %q: %w", cmd.FullName(), errNoCompletion))
	}
}

// setupCompletionCommand is a no-op: with the urfave_cli_no_completion tag
// the hidden "completion" command is not compiled in. Requesting it is a
// programming error and panics.
func (cmd *Command) setupCompletionCommand(isRoot bool) {
	if isRoot && cmd.EnableShellCompletion || cmd.ConfigureShellCompletionCommand != nil {
		panic(fmt.Errorf("command %q: %w", cmd.FullName(), errNoCompletion))
	}
}

func checkShellCompleteFlag(c *Command, arguments []string) (bool, []string) {
	return false, arguments
}

func shouldRunCompletion(cmd *Command) bool {
	return false
}

func runCompletion(ctx context.Context, cmd *Command) {}
