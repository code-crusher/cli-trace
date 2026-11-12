//go:build !urfave_cli_no_completion

package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompletionHelp(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "short help flag",
			args: []string{"foo", completionCommandName, "-h"},
		},
		{
			name: "long help flag",
			args: []string{"foo", completionCommandName, "--help"},
		},
		{
			name: "completion bash short help flag",
			args: []string{"foo", completionCommandName, "bash", "-h"},
		},
		{
			name: "completion bash long help flag",
			args: []string{"foo", completionCommandName, "bash", "--help"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out := &bytes.Buffer{}

			cmd := &Command{
				EnableShellCompletion: true,
				Writer:                out,
				Flags: []Flag{
					&StringFlag{
						Name:     "required-flag",
						Required: true,
					},
				},
			}

			r := require.New(t)

			r.NoError(cmd.Run(buildTestContext(t), test.args))
			r.Contains(out.String(), "USAGE")
			r.NotContains(out.String(), "GLOBAL OPTIONS")
		})
	}
}

func TestCompletionDisable(t *testing.T) {
	cmd := &Command{}

	err := cmd.Run(buildTestContext(t), []string{"foo", completionCommandName})
	assert.Error(t, err, "Expected error for no help topic for completion")
}

func TestCompletionEnable(t *testing.T) {
	out := &bytes.Buffer{}

	cmd := &Command{
		EnableShellCompletion: true,
		Writer:                out,
		Flags: []Flag{
			&StringFlag{
				Name:     "goo",
				Required: true,
			},
		},
	}

	r := require.New(t)
	r.NoError(cmd.Run(buildTestContext(t), []string{"foo", completionCommandName}))
	r.Contains(out.String(), "USAGE")
}

func TestCompletionEnableDiffCommandName(t *testing.T) {
	out := &bytes.Buffer{}

	cmd := &Command{
		EnableShellCompletion:      true,
		ShellCompletionCommandName: "junky",
		Writer:                     out,
	}

	r := require.New(t)
	r.NoError(cmd.Run(buildTestContext(t), []string{"foo", "junky"}))
	r.Contains(out.String(), "USAGE")
	// the setup instructions must use the renamed command
	r.Contains(out.String(), "source <(foo junky bash)")
	r.Contains(out.String(), "foo junky fish > ~/.config/fish/completions/foo.fish")
	r.NotContains(out.String(), "foo completion")
}

func TestCompletionShell(t *testing.T) {
	for k := range shellCompletions {
		out := &bytes.Buffer{}

		t.Run(k, func(t *testing.T) {
			cmd := &Command{
				EnableShellCompletion: true,
				Writer:                out,
			}

			r := require.New(t)

			r.NoError(cmd.Run(buildTestContext(t), []string{"foo", completionCommandName, k}))
			r.NotEmpty(out.String(), "Expected non-empty completion output for shell %q", k)
		})
	}
}

func TestCompletionSubcommandOrder(t *testing.T) {
	// The completion subcommands must appear in a deterministic order so that
	// help output (and docs generated from it) does not change between runs.
	// Previously they were built by iterating a map, whose order Go randomizes.
	want := []string{"bash", "zsh", "fish", "pwsh"}

	// Build several times to guard against intra-process variation.
	for range 10 {
		cmd := buildCompletionCommand("foo", completionCommandName)

		got := make([]string, 0, len(cmd.Commands))
		for _, sub := range cmd.Commands {
			got = append(got, sub.Name)
		}

		assert.Equal(t, want, got)
	}

	// Every shell in shellCompletions must be represented in the ordered list.
	assert.Len(t, completionShells, len(shellCompletions))
	for shell := range shellCompletions {
		assert.Contains(t, completionShells, shell)
	}
}

func TestCompletionBashNoShebang(t *testing.T) {
	// Regression test for https://github.com/urfave/cli/issues/2259
	// Bash completion scripts are sourced, not executed, so they must not
	// start with a `#!` shebang (flagged by Debian lintian as
	// `bash-completion-with-hashbang`).

	cmd := &Command{
		EnableShellCompletion: true,
	}

	r := require.New(t)

	bashRender := shellCompletions["bash"]
	r.NotNil(bashRender, "bash completion renderer should exist")

	output, err := bashRender(cmd, "myapp")
	r.NoError(err)
	r.NotEmpty(output, "bash completion output should not be empty")
	r.False(strings.HasPrefix(output, "#!"), "bash completion should not start with a shebang")
}

func TestCompletionBashAppendsSpace(t *testing.T) {
	// Regression test for https://github.com/urfave/cli/issues/2332
	// Do not register bash completions with `-o nospace`: after a command or
	// subcommand completion, Bash should append a space so the next word can be
	// completed without manually typing one.

	cmd := &Command{
		EnableShellCompletion: true,
	}

	r := require.New(t)

	bashRender := shellCompletions["bash"]
	r.NotNil(bashRender, "bash completion renderer should exist")

	output, err := bashRender(cmd, "myapp")
	r.NoError(err)
	r.NotContains(output, "-o nospace", "bash completion should append spaces after completed words")
	r.Contains(output, "complete -o bashdefault -o default -F __myapp_bash_autocomplete myapp")
}

func TestCompletionBashGreedyColonParsing(t *testing.T) {
	// Regression test for https://github.com/urfave/cli/issues/2335
	// The bash completion template uses fmt.Sprintf, so
	// literal "%" in the template must be escaped as "%%". The token
	// extraction must use the greedy ${line%%:*} (double %%) to split on
	// the *first* colon. A single % would use ${line%:*} which splits on
	// the *last* colon, breaking descriptions that contain colons
	// (e.g. "export:Export configs such as: compose-config").

	cmd := &Command{
		EnableShellCompletion: true,
	}

	r := require.New(t)

	bashRender := shellCompletions["bash"]
	r.NotNil(bashRender, "bash completion renderer should exist")

	output, err := bashRender(cmd, "myapp")
	r.NoError(err)

	// After fmt.Sprintf, the rendered script must contain ${line%%:*}
	// (greedy match) not ${line%:*} (non-greedy match).
	r.Contains(output, `${line%%:*}`, "token extraction should use greedy %% to match first colon")
	r.NotContains(output, `${line%:*}`, "token extraction must not use non-greedy single % (splits on last colon)")
}

func TestCompletionFishFormat(t *testing.T) {
	// Regression test for https://github.com/urfave/cli/issues/2285
	// Fish completion was broken due to incorrect format specifiers

	cmd := &Command{
		Name:                  "myapp",
		EnableShellCompletion: true,
	}

	r := require.New(t)

	// Test the fish shell completion renderer directly
	fishRender := shellCompletions["fish"]
	r.NotNil(fishRender, "fish completion renderer should exist")

	output, err := fishRender(cmd, "myapp")
	r.NoError(err)

	// Verify the function name is correctly formatted
	r.Contains(output, "function __myapp_perform_completion", "function name should contain app name")

	// Verify no format errors (like %! or (string=) which indicate broken fmt.Sprintf)
	r.NotContains(output, "%!", "output should not contain format errors")
	r.NotContains(output, "(string=", "output should not contain invalid fish syntax")

	// Verify the complete commands reference the app correctly
	r.Contains(output, "complete -c myapp", "complete command should reference app name")
	r.Contains(output, "(__myapp_perform_completion)", "completion function should be registered")
}

func TestCompletionFishOmitsPositionalTokenFromDynamicCompletion(t *testing.T) {
	cmd := &Command{
		Name:                  "myapp",
		EnableShellCompletion: true,
	}

	r := require.New(t)

	fishRender := shellCompletions["fish"]
	r.NotNil(fishRender, "fish completion renderer should exist")

	output, err := fishRender(cmd, "myapp")
	r.NoError(err)

	r.Contains(output, `if string match -q -- "-*" $lastArg`)
	r.Contains(output, "set results ($args[1] $args[2..-1] $lastArg --generate-shell-completion 2> /dev/null)")
	r.Contains(output, "set results ($args[1] $args[2..-1] --generate-shell-completion 2> /dev/null)")
}

func TestCompletionFishKeepsSuggestionsStartingWithAppName(t *testing.T) {
	cmd := &Command{
		Name:                  "app",
		EnableShellCompletion: true,
	}

	r := require.New(t)

	output, err := shellCompletions["fish"](cmd, "app")
	r.NoError(err)

	// A subcommand like "apply" for an app named "app" has to reach fish too
	r.NotContains(output, `string match -q -- "app*"`)
	r.Contains(output, `set -l parts (string split -m 1 ":" -- "$line")`)
}

func TestCompletionBashOmitsPositionalTokenFromDynamicCompletion(t *testing.T) {
	cmd := &Command{
		Name:                  "myapp",
		EnableShellCompletion: true,
	}

	r := require.New(t)

	bashRender := shellCompletions["bash"]
	r.NotNil(bashRender, "bash completion renderer should exist")

	output, err := bashRender(cmd, "myapp")
	r.NoError(err)

	r.Contains(output, `if [[ "${current_word}" == "-"* ]]; then`)
	r.Contains(output, `printf '%s %s --generate-shell-completion' "${words_before_cursor[*]}" "${current_word}"`)
	r.Contains(output, `printf '%s --generate-shell-completion' "${words_before_cursor[*]}"`)
}

func TestCompletionBashDoesNotExpandCandidates(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash completion is not tested on Windows")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not found")
	}

	r := require.New(t)

	script, err := shellCompletions["bash"](&Command{}, "app")
	r.NoError(err)

	// The fake app prints a candidate that runs touch if bash expands it.
	cmd := exec.Command(bash, "-c", `
app() { echo build; echo '$(touch${IFS}expanded)'; }
# Stub for bash-completion's _comp_initialize.
_comp_initialize() { cur=${COMP_WORDS[COMP_CWORD]}; words=("${COMP_WORDS[@]}"); cword=$COMP_CWORD; }
eval "$1"
# COMP_TYPE 9 is the first TAB, 63 the second (the listing).
COMP_WORDS=(app '') COMP_CWORD=1
for COMP_TYPE in 9 63; do
	__app_bash_autocomplete
	printf '%s\n' "${COMPREPLY[@]}"
done
`, "bash", script)
	cmd.Dir = t.TempDir()

	out, err := cmd.Output()
	r.NoError(err)
	r.Equal(strings.Repeat("build\n$(touch${IFS}expanded)\n", 2), string(out))
	r.NoFileExists(filepath.Join(cmd.Dir, "expanded"))
}

func TestCompletionSubcommand(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		contains    string
		msg         string
		msgArgs     []any
		notContains bool
		// wantNoAction asserts that the command action must not run, even
		// though shell completion is requested (https://github.com/urfave/cli/issues/1993).
		wantNoAction bool
	}{
		{
			name:     "subcommand general completion",
			args:     []string{"foo", "bar", completionFlag},
			contains: "xyz",
			msg:      "Expected output to contain shell name %[1]q",
			msgArgs: []any{
				"xyz",
			},
		},
		{
			name:     "subcommand flag completion",
			args:     []string{"foo", "bar", "-", completionFlag},
			contains: "l1",
			msg:      "Expected output to contain shell name %[1]q",
			msgArgs: []any{
				"l1",
			},
		},
		{
			name:     "subcommand double dash shows long flags",
			args:     []string{"foo", "bar", "--", completionFlag},
			contains: "--l1",
			msg:      "Expected output to contain flag %[1]q",
			msgArgs: []any{
				"--l1",
			},
		},
		{
			name:     "sub sub command general completion",
			args:     []string{"foo", "bar", "xyz", completionFlag},
			contains: "-g",
			msg:      "Expected output to contain flag %[1]q",
			msgArgs: []any{
				"-g",
			},
			notContains: true,
		},
		{
			name:     "sub sub command flag completion",
			args:     []string{"foo", "bar", "xyz", "-", completionFlag},
			contains: "-g",
			msg:      "Expected output to contain flag %[1]q",
			msgArgs: []any{
				"-g",
			},
		},
		{
			name:     "sub sub command double dash shows flags",
			args:     []string{"foo", "bar", "xyz", "--", completionFlag},
			contains: "--help",
			msg:      "Expected output to contain flag %[1]q",
			msgArgs: []any{
				"--help",
			},
		},
		{
			name:     "sub sub command no completion extra args",
			args:     []string{"foo", "bar", "xyz", "--", "sargs", completionFlag},
			contains: "-g",
			msg:      "Expected output to contain flag %[1]q",
			msgArgs: []any{
				"-g",
			},
			notContains:  true,
			wantNoAction: true,
		},
		{
			name:     "subcommand partial double dash flag completion",
			args:     []string{"foo", "bar", "--l", completionFlag},
			contains: "--l1",
			msg:      "Expected output to contain flag %[1]q",
			msgArgs: []any{
				"--l1",
			},
		},
		{
			name:     "sub sub command partial double dash flag completion",
			args:     []string{"foo", "bar", "xyz", "--he", completionFlag},
			contains: "--help",
			msg:      "Expected output to contain flag %[1]q",
			msgArgs: []any{
				"--help",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			actionRan := false

			cmd := &Command{
				EnableShellCompletion: true,
				Writer:                out,
				Commands: []*Command{
					{
						Name: "bar",
						Flags: []Flag{
							&StringFlag{
								Name: "l1",
							},
						},
						Action: func(ctx context.Context, c *Command) error {
							actionRan = true
							return nil
						},
						Commands: []*Command{
							{
								Name: "xyz",
								Flags: []Flag{
									&StringFlag{
										Name: "g",
										Aliases: []string{
											"t",
										},
									},
								},
								Action: func(ctx context.Context, c *Command) error {
									actionRan = true
									return nil
								},
							},
						},
					},
				},
			}

			r := require.New(t)

			r.NoError(cmd.Run(buildTestContext(t), test.args))
			if test.notContains {
				r.NotContainsf(out.String(), test.contains, test.msg, test.msgArgs...)
			} else {
				r.Containsf(out.String(), test.contains, test.msg, test.msgArgs...)
			}
			if test.wantNoAction {
				r.False(actionRan, "command action must not run for a completion request")
			}
		})
	}
}

func TestCompletionAfterCompleteRootFlag(t *testing.T) {
	// The root command completes the arguments passed to Run, which still
	// hold the flags. Flag suggestions are printed for the last argument,
	// and a flag that needs nothing more also lists the subcommands. The
	// shell keeps only the suggestions that match the word being typed.

	tests := []struct {
		name string
		arg  string
		want []string
	}{
		{name: "bool flag", arg: "--verbose", want: []string{"sub"}},
		{name: "inverse bool flag", arg: "--no-color", want: []string{"sub"}},
		{name: "value after equals", arg: "--format=json", want: []string{"sub"}},
		{name: "flag waiting for a value", arg: "--format", want: []string{}},
		{name: "start of a longer flag", arg: "--debug", want: []string{"--debug-level", "sub"}},
		{name: "partial flag", arg: "--verb", want: []string{"--verbose"}},
		{name: "short alias", arg: "-v", want: []string{"sub"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			cmd := &Command{
				EnableShellCompletion: true,
				HideHelp:              true,
				Writer:                out,
				Flags: []Flag{
					&BoolFlag{Name: "verbose", Aliases: []string{"v"}},
					&BoolWithInverseFlag{Name: "color"},
					&StringFlag{Name: "format"},
					&BoolFlag{Name: "debug"},
					&StringFlag{Name: "debug-level"},
				},
				Commands: []*Command{
					{Name: "sub"},
				},
			}

			r := require.New(t)
			r.NoError(cmd.Run(buildTestContext(t), []string{"foo", test.arg, completionFlag}))
			r.Equal(test.want, strings.Fields(out.String()))
		})
	}
}

func TestCompletionAfterDoubleDashNeverRunsAction(t *testing.T) {
	// Regression test for https://github.com/urfave/cli/issues/1993:
	// pressing tab on a command line that holds a "--" must never execute
	// the command action, and nothing is suggested past the "--" because
	// only positional arguments are accepted after it.

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "root command past double dash",
			args: []string{"foo", "--", "somearg", completionFlag},
		},
		{
			name: "root command past double dash multiple words",
			args: []string{"foo", "--", "bar", "baz", completionFlag},
		},
		{
			name: "subcommand past double dash",
			args: []string{"foo", "sub", "--", "somearg", completionFlag},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			actionRan := false

			cmd := &Command{
				EnableShellCompletion: true,
				Writer:                out,
				Action: func(ctx context.Context, c *Command) error {
					actionRan = true
					return nil
				},
				Commands: []*Command{
					{
						Name: "sub",
						Action: func(ctx context.Context, c *Command) error {
							actionRan = true
							return nil
						},
					},
				},
			}

			r := require.New(t)
			r.NoError(cmd.Run(buildTestContext(t), test.args))
			r.Empty(out.String(), "no suggestions expected past a --")
			r.False(actionRan, "command action must not run for a completion request")
		})
	}
}

func TestCompletionAfterDoubleDashDoesNotLeakToNextRun(t *testing.T) {
	// A Command answering several completion requests must answer each one
	// on its own terms: a request past a "--" must not make the next
	// request behave as though it were past one too.
	out := &bytes.Buffer{}

	cmd := &Command{
		EnableShellCompletion: true,
		Writer:                out,
		Flags: []Flag{
			&BoolFlag{
				Name: "verbose",
			},
		},
		Commands: []*Command{
			{
				Name: "sub",
			},
		},
	}

	r := require.New(t)

	r.NoError(cmd.Run(buildTestContext(t), []string{"foo", "--", "somearg", completionFlag}))
	r.Empty(out.String())

	out.Reset()
	r.NoError(cmd.Run(buildTestContext(t), []string{"foo", "-", completionFlag}))
	r.Contains(out.String(), "-verbose")
}

func TestCompletionUsesRunArguments(t *testing.T) {
	// Completion must use the arguments passed to Run, not os.Args, which
	// holds the go test flags here
	// (https://github.com/urfave/cli/issues/2486).
	out := &bytes.Buffer{}

	cmd := &Command{
		EnableShellCompletion: true,
		Writer:                out,
		Commands: []*Command{
			{
				Name: "sub",
				Flags: []Flag{
					&BoolFlag{
						Name: "verbose",
					},
				},
				// a custom completer may pass a Command without a parent
				ShellComplete: func(ctx context.Context, cmd *Command) {
					DefaultCompleteWithFlags(ctx, &Command{Flags: cmd.Flags, Writer: cmd.Root().Writer})
				},
			},
		},
	}

	r := require.New(t)
	r.NoError(cmd.Run(buildTestContext(t), []string{"foo", completionFlag}))
	r.Contains(out.String(), "sub")

	out.Reset()
	r.NoError(cmd.Run(buildTestContext(t), []string{"foo", "sub", "--v", completionFlag}))
	r.Equal("--verbose\n", out.String())
}

func TestCompletionSubcommandCustomShellComplete(t *testing.T) {
	out := &bytes.Buffer{}

	cmd := &Command{
		EnableShellCompletion: true,
		Writer:                out,
		Commands: []*Command{
			{
				Name: "index",
				Commands: []*Command{
					{
						Name: "show",
						ShellComplete: func(ctx context.Context, cmd *Command) {
							fmt.Fprintln(cmd.Root().Writer, "custom-index")
						},
						Action: func(ctx context.Context, cmd *Command) error { return nil },
					},
				},
			},
		},
	}

	r := require.New(t)
	r.NoError(cmd.Run(buildTestContext(t), []string{"foo", "index", "show", completionFlag}))
	r.Equal("custom-index\n", out.String())
}

func TestCompletionAfterShortFlag(t *testing.T) {
	// "-c" is the short flag itself, so it must not suggest the long flags
	// whose names start with "c". With a space after it, the shells would
	// offer "--config" as the value of "-c".

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{name: "short flag", args: []string{"foo", "-c"}, want: []string{}},
		{name: "short flag on a subcommand", args: []string{"foo", "sub", "-c"}, want: []string{}},
		{name: "start of long flags", args: []string{"foo", "-co"}, want: []string{"--config", "--count"}},
		{name: "start of long flags after two dashes", args: []string{"foo", "--c"}, want: []string{"--config", "--count"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			cmd := &Command{
				EnableShellCompletion: true,
				HideHelp:              true,
				Writer:                out,
				Flags: []Flag{
					&StringFlag{Name: "config", Aliases: []string{"c"}},
					&IntFlag{Name: "count"},
				},
				Commands: []*Command{
					{Name: "sub"},
				},
			}

			r := require.New(t)
			r.NoError(cmd.Run(buildTestContext(t), append(test.args, completionFlag)))
			r.Equal(test.want, strings.Fields(out.String()))
		})
	}
}

func TestCompletionMultilineUsage(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "command usage",
			args:     []string{"foo", "bar", completionFlag},
			expected: "baz:runs baz\nqux:runs qux\n",
		},
		{
			name:     "flag usage",
			args:     []string{"foo", "bar", "--", completionFlag},
			expected: "--level:sets the level\n--name:sets the name\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			cmd := &Command{
				EnableShellCompletion: true,
				Writer:                out,
				Commands: []*Command{
					{
						Name:     "bar",
						HideHelp: true,
						Flags: []Flag{
							&StringFlag{Name: "level", Usage: "sets the level\nto one of low or high"},
							&StringFlag{Name: "name", Usage: "sets the name\r\non Windows"},
						},
						Commands: []*Command{
							{Name: "baz", Usage: "runs baz\n\nand then stops"},
							{Name: "qux", Usage: "runs qux\r\non Windows"},
						},
					},
				},
			}

			r := require.New(t)
			r.NoError(cmd.Run(buildTestContext(t), test.args))
			r.Equal(test.expected, out.String())
		})
	}
}

func TestCompletionRunsBeforeChain(t *testing.T) {
	type contextKey struct{}

	out := &bytes.Buffer{}
	cmd := &Command{
		EnableShellCompletion: true,
		Writer:                out,
		Before: func(ctx context.Context, cmd *Command) (context.Context, error) {
			return context.WithValue(ctx, contextKey{}, "ready"), nil
		},
		Commands: []*Command{
			{
				Name: "index",
				Commands: []*Command{
					{
						Name: "show",
						ShellComplete: func(ctx context.Context, cmd *Command) {
							fmt.Fprintln(cmd.Root().Writer, ctx.Value(contextKey{}))
						},
						Action: func(ctx context.Context, cmd *Command) error { return nil },
					},
				},
			},
		},
	}

	r := require.New(t)
	r.NoError(cmd.Run(buildTestContext(t), []string{"foo", "index", "show", completionFlag}))
	r.Equal("ready\n", out.String())
}

func TestCompletionReturnsBeforeError(t *testing.T) {
	beforeErr := errors.New("load config")
	completed := false

	cmd := &Command{
		EnableShellCompletion: true,
		Writer:                io.Discard,
		Before: func(ctx context.Context, cmd *Command) (context.Context, error) {
			return nil, beforeErr
		},
		ShellComplete: func(ctx context.Context, cmd *Command) {
			completed = true
		},
	}

	err := cmd.Run(buildTestContext(t), []string{"foo", completionFlag})

	require.ErrorIs(t, err, beforeErr)
	assert.False(t, completed)
}

func TestCompletionInvalidShell(t *testing.T) {
	cmd := &Command{
		EnableShellCompletion: true,
	}

	unknownShellName := "junky-sheell"
	err := cmd.Run(buildTestContext(t), []string{"foo", completionCommandName, unknownShellName})
	assert.ErrorContains(t, err, fmt.Sprintf("No help topic for '%s'", unknownShellName))
}

func TestCompletionShellRenderError(t *testing.T) {
	unknownShellName := "junky-sheell"

	enableError := true
	shellCompletions[unknownShellName] = func(c *Command, appName string) (string, error) {
		if enableError {
			return "", fmt.Errorf("can't do completion")
		}
		return "something", nil
	}
	// buildCompletionCommand only turns shells listed in completionShells into
	// subcommands, so register the injected shell there too (restoring the
	// original slice afterward) for it to be reachable.
	defer func(orig []string) { completionShells = orig }(completionShells)
	completionShells = append(completionShells, unknownShellName)
	defer func() {
		delete(shellCompletions, unknownShellName)
	}()

	cmd := &Command{
		EnableShellCompletion: true,
	}

	err := cmd.Run(buildTestContext(t), []string{"foo", completionCommandName, unknownShellName})
	assert.ErrorContains(t, err, "can't do completion")
}

type mockWriter struct {
	err error
}

func (mw *mockWriter) Write(p []byte) (int, error) {
	if mw.err != nil {
		return 0, mw.err
	}
	return len(p), nil
}

func TestCompletionShellWriteError(t *testing.T) {
	shellName := "mock-shell"
	shellCompletions[shellName] = func(c *Command, appName string) (string, error) {
		return "something", nil
	}
	// buildCompletionCommand only turns shells listed in completionShells into
	// subcommands, so register the injected shell there too (restoring the
	// original slice afterward) for it to be reachable.
	defer func(orig []string) { completionShells = orig }(completionShells)
	completionShells = append(completionShells, shellName)
	defer func() {
		delete(shellCompletions, shellName)
	}()

	cmd := &Command{
		EnableShellCompletion: true,
		Writer:                &mockWriter{err: fmt.Errorf("writer error")},
	}

	err := cmd.Run(buildTestContext(t), []string{"foo", completionCommandName, shellName})
	assert.ErrorContains(t, err, "writer error")
}

type completionOnlyFlag struct {
	name string
}

func (f *completionOnlyFlag) String() string           { return "" }
func (f *completionOnlyFlag) Get() any                 { return nil }
func (f *completionOnlyFlag) PreParse() error          { return nil }
func (f *completionOnlyFlag) PostParse() error         { return nil }
func (f *completionOnlyFlag) Set(string, string) error { return nil }
func (f *completionOnlyFlag) Names() []string          { return []string{f.name} }
func (f *completionOnlyFlag) IsSet() bool              { return false }

func TestDefaultCompleteWithFlags(t *testing.T) {
	origArgv := os.Args
	t.Cleanup(func() { os.Args = origArgv })

	for _, tc := range []struct {
		name     string
		cmd      *Command
		argv     []string
		env      map[string]string
		expected string
	}{
		{
			name:     "empty",
			cmd:      &Command{},
			argv:     []string{"prog", "cmd"},
			env:      map[string]string{"SHELL": "bash"},
			expected: "",
		},
		{
			name: "persistent-flag-after-positional-argument",
			cmd: &Command{
				Name: "add",
				Arguments: []Argument{
					&StringArg{Name: "arg1"},
				},
				parent: &Command{
					Name: "app",
					Flags: []Flag{
						&StringFlag{Name: "config", Usage: "path to config file"},
					},
				},
			},
			argv:     []string{"app", "add", "value", "--conf", completionFlag},
			env:      map[string]string{"SHELL": "bash"},
			expected: "--config:path to config file\n",
		},
		{
			name: "typical-flag-suggestion",
			cmd: &Command{
				Flags: []Flag{
					&BoolFlag{Name: "excitement"},
					&StringFlag{Name: "hat-shape"},
				},
				parent: &Command{
					Name: "cmd",
					Flags: []Flag{
						&BoolFlag{Name: "happiness"},
						&Int64Flag{Name: "everybody-jump-on"},
					},
					Commands: []*Command{
						{Name: "putz"},
					},
				},
			},
			argv:     []string{"cmd", "--e", completionFlag},
			env:      map[string]string{"SHELL": "bash"},
			expected: "--excitement\n--everybody-jump-on\n",
		},
		{
			name: "shared-local-and-persistent-flag-is-suggested-once",
			cmd: func() *Command {
				shared := &BoolFlag{Name: "verbose"}
				return &Command{
					Flags:  []Flag{shared},
					parent: &Command{Name: "app", Flags: []Flag{shared}},
				}
			}(),
			argv:     []string{"app", "sub", "--ver", completionFlag},
			env:      map[string]string{"SHELL": "bash"},
			expected: "--verbose\n",
		},
		{
			name: "custom-flag-without-visible-flag-is-suggested",
			cmd: &Command{
				Flags: []Flag{&completionOnlyFlag{name: "custom"}},
			},
			argv:     []string{"app", "--cus", completionFlag},
			env:      map[string]string{"SHELL": "bash"},
			expected: "--custom\n",
		},
		{
			name: "mutually-exclusive-flags-are-suggested",
			cmd: &Command{
				MutuallyExclusiveFlags: []MutuallyExclusiveFlags{{
					Flags: [][]Flag{
						{&BoolFlag{Name: "alpha"}},
						{&BoolFlag{Name: "also"}},
					},
				}},
			},
			argv:     []string{"app", "--al", completionFlag},
			env:      map[string]string{"SHELL": "bash"},
			expected: "--alpha\n--also\n",
		},
		{
			name: "skip-flag-parsing-does-not-suggest-parent-flags",
			cmd: &Command{
				SkipFlagParsing: true,
				parent: &Command{
					Name:  "app",
					Flags: []Flag{&BoolFlag{Name: "root-flag"}},
				},
			},
			argv:     []string{"app", "sub", "--r", completionFlag},
			env:      map[string]string{"SHELL": "bash"},
			expected: "",
		},
		{
			name: "typical-flag-suggestion-hidden-bool",
			cmd: &Command{
				Flags: []Flag{
					&BoolFlag{Name: "excitement", Hidden: true},
					&StringFlag{Name: "hat-shape"},
				},
				parent: &Command{
					Name: "cmd",
					Flags: []Flag{
						&BoolFlag{Name: "happiness"},
						&Int64Flag{Name: "everybody-jump-on"},
					},
					Commands: []*Command{
						{Name: "putz"},
					},
				},
			},
			argv:     []string{"cmd", "--e", completionFlag},
			env:      map[string]string{"SHELL": "bash"},
			expected: "--everybody-jump-on\n",
		},
		{
			name: "typical-flag-suggestion-hidden-non-bool",
			cmd: &Command{
				Flags: []Flag{
					&StringFlag{Name: "excellent", Hidden: true},
					&StringFlag{Name: "excitement"},
				},
				parent: &Command{
					Name: "cmd",
					Flags: []Flag{
						&BoolFlag{Name: "happiness"},
						&Int64Flag{Name: "everybody-jump-on"},
					},
					Commands: []*Command{
						{Name: "putz"},
					},
				},
			},
			argv:     []string{"cmd", "--e", completionFlag},
			env:      map[string]string{"SHELL": "bash"},
			expected: "--excitement\n--everybody-jump-on\n",
		},
		{
			name: "typical-flag-suggestion-hidden-bool-with-inverse",
			cmd: &Command{
				Flags: []Flag{
					&BoolWithInverseFlag{Name: "excellent", Hidden: true},
					&BoolWithInverseFlag{Name: "excitement"},
				},
				parent: &Command{
					Name: "cmd",
					Flags: []Flag{
						&BoolFlag{Name: "happiness"},
						&Int64Flag{Name: "everybody-jump-on"},
					},
					Commands: []*Command{
						{Name: "putz"},
					},
				},
			},
			argv:     []string{"cmd", "--e", completionFlag},
			env:      map[string]string{"SHELL": "bash"},
			expected: "--excitement\n--everybody-jump-on\n",
		},
		{
			name: "flag-suggestion-double-dash-shows-all-flags",
			cmd: &Command{
				Flags: []Flag{
					&BoolFlag{Name: "excitement"},
					&StringFlag{Name: "hat-shape"},
				},
				parent: &Command{
					Name: "cmd",
					Flags: []Flag{
						&BoolFlag{Name: "happiness"},
						&Int64Flag{Name: "everybody-jump-on"},
					},
					Commands: []*Command{
						{Name: "putz"},
					},
				},
			},
			argv:     []string{"cmd", "--e", "--", completionFlag},
			env:      map[string]string{"SHELL": "bash"},
			expected: "--excitement\n--hat-shape\n--happiness\n--everybody-jump-on\n",
		},
		{
			name: "typical-command-suggestion",
			cmd: &Command{
				Name: "putz",
				Commands: []*Command{
					{Name: "futz"},
				},
				Flags: []Flag{
					&BoolFlag{Name: "excitement"},
					&StringFlag{Name: "hat-shape"},
				},
				parent: &Command{
					Name: "cmd",
					Flags: []Flag{
						&BoolFlag{Name: "happiness"},
						&Int64Flag{Name: "everybody-jump-on"},
					},
				},
			},
			argv:     []string{"cmd", completionFlag},
			env:      map[string]string{"SHELL": "bash"},
			expected: "futz\n",
		},
		{
			name: "autocomplete-with-spaces",
			cmd: &Command{
				Name: "putz",
				Commands: []*Command{
					{Name: "help"},
				},
				Flags: []Flag{
					&BoolFlag{Name: "excitement"},
					&StringFlag{Name: "hat-shape"},
				},
				parent: &Command{
					Name: "cmd",
					Flags: []Flag{
						&BoolFlag{Name: "happiness"},
						&Int64Flag{Name: "everybody-jump-on"},
					},
				},
			},
			argv:     []string{"cmd", "--url", "http://localhost:8000", "h", completionFlag},
			env:      map[string]string{"SHELL": "bash"},
			expected: "help\n",
		},
		{
			name: "zsh-autocomplete-with-flag-descriptions",
			cmd: &Command{
				Name: "putz",
				Flags: []Flag{
					&BoolFlag{Name: "excitement", Usage: "an exciting flag"},
					&StringFlag{Name: "hat-shape"},
				},
				parent: &Command{
					Name: "cmd",
					Flags: []Flag{
						&BoolFlag{Name: "happiness"},
						&Int64Flag{Name: "everybody-jump-on"},
					},
				},
			},
			argv:     []string{"cmd", "putz", "-e", completionFlag},
			env:      map[string]string{"SHELL": "zsh"},
			expected: "--excitement:an exciting flag\n--everybody-jump-on\n",
		},
		{
			name: "zsh-autocomplete-with-empty-flag-descriptions",
			cmd: &Command{
				Name: "putz",
				Flags: []Flag{
					&BoolFlag{Name: "excitement"},
					&StringFlag{Name: "hat-shape"},
				},
				parent: &Command{
					Name: "cmd",
					Flags: []Flag{
						&BoolFlag{Name: "happiness"},
						&Int64Flag{Name: "everybody-jump-on"},
					},
				},
			},
			argv:     []string{"cmd", "putz", "-e", completionFlag},
			env:      map[string]string{"SHELL": "zsh"},
			expected: "--excitement\n--everybody-jump-on\n",
		},
	} {
		t.Run(tc.name, func(ct *testing.T) {
			writer := &bytes.Buffer{}
			rootCmd := tc.cmd.Root()
			rootCmd.Writer = writer

			os.Args = tc.argv
			for k, v := range tc.env {
				ct.Setenv(k, v)
			}
			tc.cmd.parsedArgs = &stringSliceArgs{
				tc.argv[1:],
			}
			f := DefaultCompleteWithFlags
			f(buildTestContext(ct), tc.cmd)

			written := writer.String()

			assert.Equal(ct, tc.expected, written, "written help does not match")
		})
	}
}

func Test_checkShellCompleteFlag(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                string
		cmd                 *Command
		arguments           []string
		wantShellCompletion bool
		wantPastDoubleDash  bool
		wantArgs            []string
	}{
		{
			name:                "disable-shell-completion",
			arguments:           []string{completionFlag},
			cmd:                 &Command{},
			wantShellCompletion: false,
			wantArgs:            []string{completionFlag},
		},
		{
			name:      "child-disable-shell-completion",
			arguments: []string{completionFlag},
			cmd: &Command{
				parent: &Command{},
			},
			wantShellCompletion: false,
			wantArgs:            []string{completionFlag},
		},
		{
			name:      "last argument isn't --generate-shell-completion",
			arguments: []string{"foo"},
			cmd: &Command{
				EnableShellCompletion: true,
			},
			wantShellCompletion: false,
			wantArgs:            []string{"foo"},
		},
		{
			name:                "arguments include double dash",
			arguments:           []string{"--", "foo", completionFlag},
			cmd:                 &Command{EnableShellCompletion: true},
			wantShellCompletion: true,
			wantPastDoubleDash:  true,
			wantArgs:            []string{"--", "foo"},
		},
		{
			name:      "shell completion",
			arguments: []string{"foo", completionFlag},
			cmd: &Command{
				EnableShellCompletion: true,
			},
			wantShellCompletion: true,
			wantArgs:            []string{"foo"},
		},
		{
			name:      "double dash is the token being completed",
			arguments: []string{"foo", "--", completionFlag},
			cmd: &Command{
				EnableShellCompletion: true,
			},
			wantShellCompletion: true,
			wantArgs:            []string{"foo", "--"},
		},
		{
			name:      "no arguments at all",
			arguments: []string{},
			cmd: &Command{
				EnableShellCompletion: true,
			},
			wantShellCompletion: false,
			wantArgs:            []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			shellCompletion, args := checkShellCompleteFlag(tt.cmd, tt.arguments)
			assert.Equal(t, tt.wantShellCompletion, shellCompletion)
			assert.Equal(t, tt.wantPastDoubleDash, tt.cmd.shellCompletionPastDoubleDash)
			assert.Equal(t, tt.wantArgs, args)
		})
	}
}

func TestCliArgContainsFlag(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		contains bool
	}{
		{
			name: "",
			args: []string{},
		},
		{
			name: "f",
			args: []string{},
		},
		{
			name: "f",
			args: []string{"g", "foo", "f"},
		},
		{
			name:     "f",
			args:     []string{"-f", "foo", "f"},
			contains: true,
		},
		{
			name:     "f",
			args:     []string{"g", "-f", "f"},
			contains: true,
		},
		{
			name:     "fh",
			args:     []string{"g", "f", "--fh"},
			contains: true,
		},
		{
			name: "fhg",
			args: []string{"-fhg", "f", "fh"},
		},
		{
			name:     "fhg",
			args:     []string{"--fhg", "f", "fh"},
			contains: true,
		},
	}

	for _, test := range tests {
		if test.contains {
			assert.True(t, cliArgContains(test.name, test.args))
		} else {
			assert.False(t, cliArgContains(test.name, test.args))
		}
	}
}
