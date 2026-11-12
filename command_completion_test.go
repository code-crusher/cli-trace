//go:build !urfave_cli_no_completion

package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommand_Run_CustomShellCompleteAcceptsMalformedFlags(t *testing.T) {
	cases := []struct {
		testArgs    *stringSliceArgs
		expectedOut string
	}{
		{testArgs: &stringSliceArgs{v: []string{"--undefined"}}, expectedOut: "found 1 args"},
		{testArgs: &stringSliceArgs{v: []string{"--number"}}, expectedOut: "found 1 args"},
		{testArgs: &stringSliceArgs{v: []string{"--number", "forty-two"}}, expectedOut: "found 0 args"},
		{testArgs: &stringSliceArgs{v: []string{"--number", "42"}}, expectedOut: "found 0 args"},
		{testArgs: &stringSliceArgs{v: []string{"--number", "42", "newArg"}}, expectedOut: "found 1 args"},
		{testArgs: &stringSliceArgs{v: []string{"-an"}}, expectedOut: "found 1 args"},
		{testArgs: &stringSliceArgs{v: []string{"-ax"}}, expectedOut: "found 1 args"},
		{testArgs: &stringSliceArgs{v: []string{"-na"}}, expectedOut: "found 1 args"},
		{testArgs: &stringSliceArgs{v: []string{"-an", "42"}}, expectedOut: "found 0 args"},
	}

	for _, c := range cases {
		t.Run(strings.Join(c.testArgs.Slice(), " "), func(t *testing.T) {
			out := &bytes.Buffer{}
			cmd := &Command{
				Writer:                 out,
				EnableShellCompletion:  true,
				UseShortOptionHandling: true,
				Name:                   "bar",
				Usage:                  "this is for testing",
				Flags: []Flag{
					&BoolFlag{Name: "a"},
					&Int64Flag{
						Name:    "number",
						Aliases: []string{"n"},
						Usage:   "A number to parse",
					},
				},
				ShellComplete: func(_ context.Context, cmd *Command) {
					fmt.Fprintf(cmd.Root().Writer, "found %[1]d args", cmd.NArg())
				},
			}

			osArgs := &stringSliceArgs{v: []string{"bar"}}
			osArgs.v = append(osArgs.v, c.testArgs.Slice()...)
			osArgs.v = append(osArgs.v, completionFlag)

			r := require.New(t)

			r.NoError(cmd.Run(buildTestContext(t), osArgs.Slice()))
			r.Equal(c.expectedOut, out.String())
		})
	}
}

// TestCommand_ParseFlags_InvalidFlagAccessHandler_Completion covers the
// completion-specific case of the InvalidFlagAccessHandler table in
// command_test.go: a partial flag during shell completion must not reach
// the handler.
func TestCommand_ParseFlags_InvalidFlagAccessHandler_Completion(t *testing.T) {
	cmd := &Command{
		EnableShellCompletion: true,
		Flags:                 []Flag{&BoolFlag{Name: "verbose"}},
		Action:                func(context.Context, *Command) error { return nil },
	}

	var invalid []string
	cmd.Writer = io.Discard
	cmd.ErrWriter = io.Discard
	cmd.InvalidFlagAccessHandler = func(_ context.Context, _ *Command, name string) {
		invalid = append(invalid, name)
	}

	require.NoError(t, cmd.Run(buildTestContext(t), []string{"app", "--ver", "--generate-shell-completion"}))
	assert.Empty(t, invalid)
}

func TestCommand_BeforeAfterFuncShellCompletion(t *testing.T) {
	t.Skip("TODO: is '--generate-shell-completion' (flag) still supported?")

	counts := &opCounts{}

	cmd := &Command{
		EnableShellCompletion: true,
		Before: func(context.Context, *Command) (context.Context, error) {
			counts.Total++
			counts.Before = counts.Total
			return nil, nil
		},
		After: func(context.Context, *Command) error {
			counts.Total++
			counts.After = counts.Total
			return nil
		},
		Commands: []*Command{
			{
				Name: "sub",
				Action: func(context.Context, *Command) error {
					counts.Total++
					counts.SubCommand = counts.Total
					return nil
				},
			},
		},
		Flags: []Flag{
			&StringFlag{Name: "opt"},
		},
		Writer: io.Discard,
	}

	r := require.New(t)

	// run with the Before() func succeeding
	r.NoError(
		cmd.Run(
			buildTestContext(t),
			[]string{
				"command",
				"--opt", "succeed",
				"sub", completionFlag,
			},
		),
	)

	r.Equalf(0, counts.Before, "Before was run")
	r.Equal(0, counts.After, "After was run")
	r.Equal(0, counts.SubCommand, "SubCommand was run")
}

func TestCommand_OrderOfOperations(t *testing.T) {
	buildCmdCounts := func() (*Command, *opCounts) {
		counts := &opCounts{}

		cmd := &Command{
			EnableShellCompletion: true,
			ShellComplete: func(context.Context, *Command) {
				counts.Total++
				counts.ShellComplete = counts.Total
			},
			OnUsageError: func(context.Context, *Command, error, bool) error {
				counts.Total++
				counts.OnUsageError = counts.Total
				return errors.New("hay OnUsageError")
			},
			Writer: io.Discard,
		}

		beforeNoError := func(context.Context, *Command) (context.Context, error) {
			counts.Total++
			counts.Before = counts.Total
			return nil, nil
		}

		cmd.Before = beforeNoError
		cmd.CommandNotFound = func(context.Context, *Command, string) {
			counts.Total++
			counts.CommandNotFound = counts.Total
		}

		afterNoError := func(context.Context, *Command) error {
			counts.Total++
			counts.After = counts.Total
			return nil
		}

		cmd.After = afterNoError
		cmd.Commands = []*Command{
			{
				Name: "bar",
				Action: func(context.Context, *Command) error {
					counts.Total++
					counts.SubCommand = counts.Total
					return nil
				},
			},
		}

		cmd.Action = func(context.Context, *Command) error {
			counts.Total++
			counts.Action = counts.Total
			return nil
		}

		return cmd, counts
	}

	t.Run("on usage error", func(t *testing.T) {
		cmd, counts := buildCmdCounts()
		r := require.New(t)

		_ = cmd.Run(buildTestContext(t), []string{"command", "--nope"})
		r.Equal(1, counts.OnUsageError)
		r.Equal(1, counts.Total)
	})

	t.Run("shell complete", func(t *testing.T) {
		cmd, counts := buildCmdCounts()
		r := require.New(t)

		_ = cmd.Run(buildTestContext(t), []string{"command", completionFlag})
		r.Equal(1, counts.Before)
		r.Equal(2, counts.ShellComplete)
		r.Equal(2, counts.Total)
	})

	t.Run("nil on usage error", func(t *testing.T) {
		cmd, counts := buildCmdCounts()
		cmd.OnUsageError = nil

		_ = cmd.Run(buildTestContext(t), []string{"command", "--nope"})
		require.Equal(t, 0, counts.Total)
	})

	t.Run("before after action hooks", func(t *testing.T) {
		cmd, counts := buildCmdCounts()
		r := require.New(t)

		_ = cmd.Run(buildTestContext(t), []string{"command", "foo"})
		r.Equal(0, counts.OnUsageError)
		r.Equal(1, counts.Before)
		r.Equal(0, counts.CommandNotFound)
		r.Equal(2, counts.Action)
		r.Equal(3, counts.After)
		r.Equal(3, counts.Total)
	})

	t.Run("before with error", func(t *testing.T) {
		cmd, counts := buildCmdCounts()
		cmd.Before = func(context.Context, *Command) (context.Context, error) {
			counts.Total++
			counts.Before = counts.Total
			return nil, errors.New("hay Before")
		}

		r := require.New(t)

		_ = cmd.Run(buildTestContext(t), []string{"command", "bar"})
		r.Equal(0, counts.OnUsageError)
		r.Equal(1, counts.Before)
		r.Equal(2, counts.After)
		r.Equal(2, counts.Total)
	})

	t.Run("nil after", func(t *testing.T) {
		cmd, counts := buildCmdCounts()
		cmd.After = nil
		r := require.New(t)

		_ = cmd.Run(buildTestContext(t), []string{"command", "bar"})
		r.Equal(0, counts.OnUsageError)
		r.Equal(1, counts.Before)
		r.Equal(2, counts.SubCommand)
		r.Equal(2, counts.Total)
	})

	t.Run("after errors", func(t *testing.T) {
		cmd, counts := buildCmdCounts()
		cmd.After = func(context.Context, *Command) error {
			counts.Total++
			counts.After = counts.Total
			return errors.New("hay After")
		}

		r := require.New(t)

		err := cmd.Run(buildTestContext(t), []string{"command", "bar"})
		r.Error(err)
		r.Equal(0, counts.OnUsageError)
		r.Equal(1, counts.Before)
		r.Equal(2, counts.SubCommand)
		r.Equal(3, counts.After)
		r.Equal(3, counts.Total)
	})

	t.Run("nil commands", func(t *testing.T) {
		cmd, counts := buildCmdCounts()
		cmd.Commands = nil
		r := require.New(t)

		_ = cmd.Run(buildTestContext(t), []string{"command"})
		r.Equal(0, counts.OnUsageError)
		r.Equal(1, counts.Before)
		r.Equal(2, counts.Action)
		r.Equal(3, counts.After)
		r.Equal(3, counts.Total)
	})
}

func TestShellCompletionForIncompleteFlags(t *testing.T) {
	cmd := &Command{
		Flags: []Flag{
			&Int64Flag{
				Name: "test-completion",
			},
		},
		EnableShellCompletion: true,
		ShellComplete: func(_ context.Context, cmd *Command) {
			for _, command := range cmd.Commands {
				if command.Hidden {
					continue
				}

				for _, name := range command.Names() {
					_, _ = fmt.Fprintln(cmd.Writer, name)
				}
			}

			for _, fl := range cmd.Flags {
				for _, name := range fl.Names() {
					if name == GenerateShellCompletionFlag.Names()[0] {
						continue
					}

					switch name = strings.TrimSpace(name); len(name) {
					case 0:
					case 1:
						_, _ = fmt.Fprintln(cmd.Writer, "-"+name)
					default:
						_, _ = fmt.Fprintln(cmd.Writer, "--"+name)
					}
				}
			}
		},
		Action: func(context.Context, *Command) error {
			return fmt.Errorf("should not get here")
		},
		Writer: io.Discard,
	}

	err := cmd.Run(buildTestContext(t), []string{"", "--test-completion", completionFlag})
	assert.NoError(t, err, "app should not return an error")
}

// TestRunWithNoOsArgs_Completion mirrors the completion cases of
// TestRunWithNoOsArgs in command_test.go: an empty Name combined with
// EnableShellCompletion exercises both the setupDefaults empty-name guard
// and the buildCompletionCommand("") append together.
func TestRunWithNoOsArgs_Completion(t *testing.T) {
	for _, tst := range []struct {
		name     string
		cmd      *Command
		wantName string
	}{
		{name: "shell completion enabled", cmd: &Command{Name: "foo", EnableShellCompletion: true}, wantName: "foo"},
		{name: "no name, shell completion enabled", cmd: &Command{EnableShellCompletion: true}},
	} {
		t.Run(tst.name, func(t *testing.T) {
			called := false
			tst.cmd.Action = func(context.Context, *Command) error {
				called = true
				return nil
			}

			require.NotPanics(t, func() {
				require.NoError(t, tst.cmd.Run(buildTestContext(t), []string{}))
			})
			assert.True(t, called)
			assert.Equal(t, tst.wantName, tst.cmd.Name)
		})
	}
}
