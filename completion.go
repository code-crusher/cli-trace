//go:build !urfave_cli_no_completion

package cli

import (
	"context"
	"embed"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"unicode/utf8"
)

// completionSupported is true unless built with the urfave_cli_no_completion
// build tag. The tag removes the hidden "completion" command, the embedded
// shell scripts and the completion machinery below, so that applications
// that do not enable shell completion do not pay for it in binary size
// (see #2291).
const completionSupported = true

const (
	completionCommandName = "completion"

	// This flag is supposed to only be used by the completion script itself to generate completions on the fly.
	completionFlag = "--generate-shell-completion"
)

type renderCompletion func(cmd *Command, appName string) (string, error)

var (
	//go:embed autocomplete
	autoCompleteFS embed.FS

	// completionShells defines the order in which the shell completion
	// subcommands appear in help output. Iterating shellCompletions directly
	// would use Go's randomized map order, making the listing nondeterministic.
	// Keep this in sync with shellCompletions.
	completionShells = []string{"bash", "zsh", "fish", "pwsh"}

	shellCompletions = map[string]renderCompletion{
		"bash": func(c *Command, appName string) (string, error) {
			b, err := autoCompleteFS.ReadFile("autocomplete/bash_autocomplete")
			return fmt.Sprintf(string(b), appName), err
		},
		"zsh": func(c *Command, appName string) (string, error) {
			b, err := autoCompleteFS.ReadFile("autocomplete/zsh_autocomplete")
			return fmt.Sprintf(string(b), appName), err
		},
		"fish": func(c *Command, appName string) (string, error) {
			b, err := autoCompleteFS.ReadFile("autocomplete/fish_autocomplete")
			return fmt.Sprintf(string(b), appName), err
		},
		"pwsh": func(c *Command, appName string) (string, error) {
			b, err := autoCompleteFS.ReadFile("autocomplete/powershell_autocomplete.ps1")
			return string(b), err
		},
	}
)

const completionDescription = `Output shell completion script for bash, zsh, fish, or Powershell.
Source the output to enable completion.

# .bashrc
source <($COMMAND $COMPLETION bash)

# .zshrc
source <($COMMAND $COMPLETION zsh)

# fish
$COMMAND $COMPLETION fish > ~/.config/fish/completions/$COMMAND.fish

# Powershell
Output the script to path/to/autocomplete/$COMMAND.ps1 and run it.
`

func buildCompletionCommand(appName, name string) *Command {
	cmd := &Command{
		Name:                name,
		Hidden:              true,
		Usage:               "Output shell completion script for bash, zsh, fish, or Powershell",
		Description:         strings.NewReplacer("$COMMAND", appName, "$COMPLETION", name).Replace(completionDescription),
		isCompletionCommand: true,
	}

	for _, shell := range completionShells {
		cmd.Commands = append(cmd.Commands, buildShellCompletionSubcommand(shell, shellCompletions[shell], appName))
	}

	return cmd
}

func buildShellCompletionSubcommand(shell string, render renderCompletion, appName string) *Command {
	return &Command{
		Name:                shell,
		Usage:               fmt.Sprintf("Output %s completion script", shell),
		isCompletionCommand: true,
		Action: func(ctx context.Context, cmd *Command) error {
			completionScript, err := render(cmd, appName)
			if err != nil {
				return Exit(err, 1)
			}
			_, err = cmd.Root().Writer.Write([]byte(completionScript))
			if err != nil {
				return Exit(err, 1)
			}
			return nil
		},
	}
}

// setupShellCompletion sets the default completion function of a command.
func (cmd *Command) setupShellCompletion() {
	if cmd.ShellComplete == nil {
		tracef("setting default ShellComplete (cmd=%[1]q)", cmd.Name)
		cmd.ShellComplete = DefaultCompleteWithFlags
	}
}

// setupCompletionCommand appends the hidden command that outputs the
// completion scripts, if the command is a root that enables shell completion
// or a completion command is explicitly configured.
func (cmd *Command) setupCompletionCommand(isRoot bool) {
	if isRoot && cmd.EnableShellCompletion || cmd.ConfigureShellCompletionCommand != nil {
		name := completionCommandName
		if cmd.ShellCompletionCommandName != "" {
			tracef(
				"setting completion command name (%[1]q) from "+
					"cmd.ShellCompletionCommandName (cmd=%[2]q)",
				cmd.ShellCompletionCommandName, cmd.Name,
			)
			name = cmd.ShellCompletionCommandName
		}

		completionCommand := buildCompletionCommand(cmd.Name, name)

		tracef("appending completionCommand (cmd=%[1]q)", cmd.Name)
		cmd.appendCommand(completionCommand)
		if cmd.ConfigureShellCompletionCommand != nil {
			cmd.ConfigureShellCompletionCommand(completionCommand)
		}
	}
}

// DefaultRootCommandComplete prints the list of subcommands as the default completion method.
func DefaultRootCommandComplete(ctx context.Context, cmd *Command) {
	DefaultCompleteWithFlags(ctx, cmd)
}

// DefaultAppComplete is a backward-compatible name for DefaultRootCommandComplete.
var DefaultAppComplete = DefaultRootCommandComplete

func printCommandSuggestions(commands []*Command, writer io.Writer) {
	for _, command := range commands {
		if command.Hidden {
			continue
		}
		if usage := completionUsage(command.Usage); len(usage) > 0 {
			_, _ = fmt.Fprintf(writer, "%s:%s\n", command.Name, usage)
		} else {
			_, _ = fmt.Fprintf(writer, "%s\n", command.Name)
		}
	}
}

// completionUsage returns the first line of usage. The shell completion
// scripts read one suggestion per line, so any further lines of a multiline
// usage would be offered as suggestions of their own.
func completionUsage(usage string) string {
	if i := strings.IndexAny(usage, "\r\n"); i >= 0 {
		return usage[:i]
	}
	return usage
}

func cliArgContains(flagName string, args []string) bool {
	for name := range strings.SplitSeq(flagName, ",") {
		name = strings.TrimSpace(name)
		count := min(utf8.RuneCountInString(name), 2)
		flag := fmt.Sprintf("%s%s", strings.Repeat("-", count), name)
		if slices.Contains(args, flag) {
			return true
		}
	}
	return false
}

func printFlagSuggestions(lastArg string, flags []Flag, writer io.Writer) {
	// Trim to handle both "-short" and "--long" flags.
	cur := strings.TrimLeft(lastArg, "-")
	// A short flag written out in full, such as "-c", is that flag and not
	// the start of a long one, so there is nothing to suggest.
	if lastArg == "-"+cur && utf8.RuneCountInString(cur) == 1 {
		for _, flag := range flags {
			if slices.Contains(flag.Names(), cur) {
				return
			}
		}
	}
	for _, flag := range flags {
		if vf, ok := flag.(VisibleFlag); ok && !vf.IsVisible() {
			continue
		}

		usage := ""
		if docFlag, ok := flag.(DocGenerationFlag); ok {
			usage = completionUsage(docFlag.GetUsage())
		}

		name := strings.TrimSpace(flag.Names()[0])
		// this will get total count utf8 letters in flag name
		// reuse this count to generate single - or -- in flag completion
		count := min(utf8.RuneCountInString(name), 2)
		// if flag name has more than one utf8 letter and last argument in cli has -- prefix then
		// skip flag completion for short flags example -v or -x
		if strings.HasPrefix(lastArg, "--") && count == 1 {
			continue
		}
		// match if last argument matches this flag and it is not repeated
		if strings.HasPrefix(name, cur) && cur != name /* && !cliArgContains(name, os.Args)*/ {
			flagCompletion := fmt.Sprintf("%s%s", strings.Repeat("-", count), name)
			if usage != "" {
				flagCompletion = fmt.Sprintf("%s:%s", flagCompletion, usage)
			}
			fmt.Fprintln(writer, flagCompletion)
		}
	}
}

// isCompleteFlag reports whether arg is a flag that needs nothing more: one
// that takes no value, or one given its value after "=".
func isCompleteFlag(arg string, flags []Flag) bool {
	name, _, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
	for _, flag := range flags {
		if !slices.Contains(flag.Names(), name) {
			continue
		}
		if hasValue {
			return true
		}
		docFlag, ok := flag.(DocGenerationFlag)
		return ok && !docFlag.TakesValue()
	}
	return false
}

func DefaultCompleteWithFlags(ctx context.Context, cmd *Command) {
	args := os.Args
	if cmd != nil && cmd.parent != nil {
		args = cmd.Args().Slice()
		tracef("running default complete with flags[%v] on command %[2]q", args, cmd.Name)
	} else if runArgs, ok := ctx.Value(runArgsKey{}).([]string); ok {
		args = runArgs
		tracef("running default complete with Run arguments[%v]", args)
	} else {
		tracef("running default complete with os.Args flags[%v]", args)
	}
	argsLen := len(args)
	lastArg := ""
	// os.Args and the arguments passed to Run retain the completion marker,
	// but a child's parsed arguments have already had it removed.
	if argsLen > 0 {
		lastArg = args[argsLen-1]
		if lastArg == completionFlag && argsLen > 1 {
			lastArg = args[argsLen-2]
		}
	}

	if lastArg == completionFlag {
		lastArg = ""
	}

	if strings.HasPrefix(lastArg, "-") {
		tracef("printing flag suggestion for flag[%v] on command %[1]q", lastArg, cmd.Name)
		flags := cmd.allFlags()
		if !cmd.SkipFlagParsing {
			for _, fl := range cmd.VisiblePersistentFlags() {
				if !hasFlag(flags, fl) {
					flags = append(flags, fl)
				}
			}
		}
		printFlagSuggestions(lastArg, flags, cmd.Root().Writer)
		// The last argument may be the flag being typed, as in "app --debug<TAB>",
		// or one before the cursor, as in "app --debug <TAB>". The bash, zsh and
		// fish scripts keep only the suggestions that match the word being typed,
		// so after a complete flag, suggest the subcommands too.
		if !isCompleteFlag(lastArg, flags) {
			return
		}
	}

	if cmd != nil {
		tracef("printing command suggestions on command %[1]q", cmd.Name)
		printCommandSuggestions(cmd.Commands, cmd.Root().Writer)
		return
	}
}

func checkShellCompleteFlag(c *Command, arguments []string) (bool, []string) {
	if (c.parent == nil && !c.EnableShellCompletion) || (c.parent != nil && !c.Root().shellCompletion) {
		return false, arguments
	}

	if len(arguments) == 0 {
		return false, arguments
	}

	pos := len(arguments) - 1
	lastArg := arguments[pos]

	if lastArg != completionFlag {
		return false, arguments
	}

	// If the token being completed is preceded by a "--", only positional
	// arguments are accepted after it, so nothing will be suggested.
	// https://unix.stackexchange.com/a/11382
	// Note: The token being completed is at position pos-1 (immediately before completionFlag).
	// A "--" at exactly that position is the token being completed, not a
	// separator, so completing "--" itself still suggests flags.
	// The request is still recognized as a completion so that the command
	// action is never executed (https://github.com/urfave/cli/issues/1993).
	if pos >= 1 && slices.Contains(arguments[:pos-1], "--") {
		c.shellCompletionPastDoubleDash = true
	}

	return true, arguments[:pos]
}

func shouldRunCompletion(cmd *Command) bool {
	tracef("checking completions on command %[1]q", cmd.Name)

	if !cmd.Root().shellCompletion {
		tracef("completion not enabled skipping %[1]q", cmd.Name)
		return false
	}

	if argsArguments := cmd.Args(); argsArguments.Present() {
		name := argsArguments.First()
		if cmd := cmd.Command(name); cmd != nil {
			// let the command handle the completion
			return false
		}
	}

	tracef("no subcommand found for completion %[1]q", cmd.Name)
	return true
}

func runCompletion(ctx context.Context, cmd *Command) {
	// Nothing is suggested past a "--": after it, only positional arguments
	// are accepted. The request is still treated as a completion so that the
	// command action is never executed (https://github.com/urfave/cli/issues/1993).
	if cmd.Root().shellCompletionPastDoubleDash {
		tracef("completion requested past double dash; suggesting nothing (cmd=%[1]q)", cmd.Name)
		return
	}
	if cmd.ShellComplete != nil {
		tracef("running shell completion func for command %[1]q", cmd.Name)
		cmd.ShellComplete(ctx, cmd)
	}
}
