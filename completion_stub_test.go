//go:build urfave_cli_no_completion

package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// With the urfave_cli_no_completion tag, all shell completion support is
// compiled out. Configuring any completion option must fail loudly instead
// of being silently ignored, since the shell scripts would call the binary
// and get nothing.

func TestNoCompletionTagPanicsOnEnableShellCompletion(t *testing.T) {
	cmd := &Command{Name: "foo", EnableShellCompletion: true}
	cmd.Action = func(context.Context, *Command) error { return nil }

	assert.PanicsWithError(
		t,
		`command "foo": shell completion is not supported when built with urfave_cli_no_completion tag`,
		func() { _ = cmd.Run(buildTestContext(t), []string{"foo"}) },
	)
}

func TestNoCompletionTagPanicsOnConfigureShellCompletionCommand(t *testing.T) {
	cmd := &Command{
		Name:                            "foo",
		ConfigureShellCompletionCommand: func(*Command) {},
	}
	cmd.Action = func(context.Context, *Command) error { return nil }

	assert.PanicsWithError(
		t,
		`command "foo": shell completion is not supported when built with urfave_cli_no_completion tag`,
		func() { _ = cmd.Run(buildTestContext(t), []string{"foo"}) },
	)
}

func TestNoCompletionTagPanicsOnShellCompleteFunc(t *testing.T) {
	cmd := &Command{
		Name:          "foo",
		ShellComplete: func(context.Context, *Command) {},
	}
	cmd.Action = func(context.Context, *Command) error { return nil }

	assert.PanicsWithError(
		t,
		`command "foo": shell completion is not supported when built with urfave_cli_no_completion tag`,
		func() { _ = cmd.Run(buildTestContext(t), []string{"foo"}) },
	)
}

// checkShellCompleteFlag is compiled to always report "not a completion
// request": the completion flag is left in the arguments, exactly as when
// shell completion is disabled at runtime.
func TestNoCompletionTagCheckShellCompleteFlag(t *testing.T) {
	cmd := &Command{EnableShellCompletion: true}
	isCompletion, args := checkShellCompleteFlag(cmd, []string{"foo", "--generate-shell-completion"})
	assert.False(t, isCompletion)
	assert.Equal(t, []string{"foo", "--generate-shell-completion"}, args)
}

func TestNoCompletionTagNoCompletionRuns(t *testing.T) {
	ran := false
	cmd := &Command{
		Name: "foo",
		ShellComplete: func(context.Context, *Command) {
			ran = true
		},
	}
	// The ShellComplete func is set without going through Run's setup, to
	// exercise shouldRunCompletion and runCompletion directly.
	require.False(t, shouldRunCompletion(cmd))
	runCompletion(t.Context(), cmd)
	assert.False(t, ran, "completion must never run with urfave_cli_no_completion")
}
