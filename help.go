package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

const (
	helpName  = "help"
	helpAlias = "h"
)

// HelpPrinterFunc prints help for the Command.
type HelpPrinterFunc func(w io.Writer, templ string, data any)

// Prints help for the Command with custom template function.
type HelpPrinterCustomFunc func(w io.Writer, templ string, data any, customFunc map[string]any)

// HelpPrinter is a function that writes the help output. If not set explicitly,
// this calls HelpPrinterCustom using only the default template functions.
//
// If custom logic for printing help is required, this function can be
// overridden. If the ExtraInfo field is defined on a Command, this function
// should not be modified, as HelpPrinterCustom will be used directly in order
// to capture the extra information.
var HelpPrinter HelpPrinterFunc = DefaultPrintHelp

// HelpPrinterCustom is a function that writes the help output. It is used as
// the default implementation of HelpPrinter, and may be called directly if
// the ExtraInfo field is set on a Command.
//
// In the default implementation, if the customFuncs argument contains a
// "wrapAt" key, which is a function which takes no arguments and returns
// an int, this int value will be used to produce a "wrap" function used
// by the default template to wrap long lines.
var HelpPrinterCustom HelpPrinterCustomFunc = DefaultPrintHelpCustom

// VersionPrinter prints the version for the root Command.
var VersionPrinter = DefaultPrintVersion

// ShowRootCommandHelp is an action that displays help for the root command.
var ShowRootCommandHelp = DefaultShowRootCommandHelp

// ShowAppHelp is a backward-compatible name for ShowRootCommandHelp.
var ShowAppHelp = ShowRootCommandHelp

// ShowCommandHelp prints help for the given command
var ShowCommandHelp = DefaultShowCommandHelp

// ShowSubcommandHelp prints help for the given subcommand
var ShowSubcommandHelp = DefaultShowSubcommandHelp

// UsageCommandHelp is the text to override the USAGE section of the help command
var UsageCommandHelp = "Shows a list of commands or help for one command"

// ArgsUsageCommandHelp is a short description of the arguments of the help command
var ArgsUsageCommandHelp = "[command]"

func buildHelpCommand(withAction bool) *Command {
	cmd := &Command{
		Name:        helpName,
		Aliases:     []string{helpAlias},
		Usage:       UsageCommandHelp,
		ArgsUsage:   ArgsUsageCommandHelp,
		HideHelp:    true,
		builtInHelp: true,
	}

	if withAction {
		cmd.Action = helpCommandAction
	}

	return cmd
}

func helpCommandAction(ctx context.Context, cmd *Command) error {
	args := cmd.Args()
	firstArg := args.First()
	helpFromFlag := ctx.Value(helpFlagKey{}) != nil

	tracef("doing help for cmd %[1]q with args %[2]q", cmd, args)

	// helpCommandAction is triggered in several ways:
	//
	//   * the command has no user-defined Action (default action fallback)
	//   * the --help / -h flag was parsed (via cmd.checkHelp())
	//   * the "help" subcommand (or "h" alias) was dispatched
	//
	// Possible invocations:
	//
	//   $ app                  # default action; show root help
	//   $ app --help / -h      # flag; show root help (ignores subsequent args)
	//   $ app help / h         # subcommand; show root help
	//   $ app help / h foo     # subcommand; show help for subcommand "foo"
	//   $ app help / h foo bar # subcommand; show help for foo's subcommand "bar"
	//   $ app --help / -h foo  # flag; show help for subcommand "foo" if it
	//                          # matches a child command; otherwise ignore it
	//   $ app foo --help / -h  # flag on subcommand; show help for "foo"
	//   $ app foo help / h     # subcommand on subcommand; show help for "foo"
	//   $ app foo (no action)  # default action on subcommand; show help for "foo"

	// Case 4. when executing a help command set the context to parent
	// to allow resolution of subsequent args. This will transform
	// $ app help foo
	//     to
	// $ app foo
	// which will then be handled as case 3
	if cmd.parent != nil && cmd.builtInHelp {
		tracef("setting cmd to cmd.parent")
		cmd = cmd.parent
	}

	// Case 4. $ app help foo
	// foo is the command for which help needs to be shown
	if firstArg != "" {
		if !helpFromFlag || cmd.Command(firstArg) != nil {
			// follow the args down as long as they name subcommands
			parent, name := cmd, firstArg
			for _, arg := range args.Tail() {
				child := parent.Command(name)
				if child == nil || child.Command(arg) == nil {
					break
				}
				parent, name = child, arg
			}
			tracef("returning ShowCommandHelp with %[1]q", name)
			return ShowCommandHelp(ctx, parent, name)
		}

		tracef("ignoring positional help argument %[1]q that is not a child command", firstArg)
	}

	// Case 1 & 2
	// Special case when running help on main app itself as opposed to individual
	// commands/subcommands
	if cmd.parent == nil {
		tracef("returning ShowRootCommandHelp")
		_ = ShowRootCommandHelp(cmd)
		return nil
	}

	// Case 3, 5
	if len(cmd.VisibleCommands()) == 0 {
		tracef("running HelpPrinter with command %[1]q", cmd.Name)
		return ShowCommandHelp(ctx, cmd.parent, cmd.Name)
	}

	tracef("running ShowSubcommandHelp")
	return ShowSubcommandHelp(cmd)
}

// ShowRootCommandHelpAndExit prints the list of subcommands and exits with exit code.
func ShowRootCommandHelpAndExit(cmd *Command, exitCode int) {
	_ = ShowRootCommandHelp(cmd)
	OsExiter(exitCode)
}

// ShowAppHelpAndExit is a backward-compatible name for ShowRootCommandHelp.
var ShowAppHelpAndExit = ShowRootCommandHelpAndExit

// DefaultShowRootCommandHelp is the default implementation of ShowRootCommandHelp.
func DefaultShowRootCommandHelp(cmd *Command) error {
	tmpl := cmd.CustomRootCommandHelpTemplate
	if tmpl == "" {
		tracef("using RootCommandHelpTemplate")
		tmpl = RootCommandHelpTemplate
	}

	if cmd.ExtraInfo == nil {
		HelpPrinter(cmd.Root().Writer, tmpl, cmd.Root())
		return nil
	}

	tracef("setting ExtraInfo in customAppData")
	customAppData := func() map[string]any {
		return map[string]any{
			"ExtraInfo": cmd.ExtraInfo,
		}
	}
	HelpPrinterCustom(cmd.Root().Writer, tmpl, cmd.Root(), customAppData())

	return nil
}

// ShowCommandHelpAndExit exits with code after showing help via ShowCommandHelp.
func ShowCommandHelpAndExit(ctx context.Context, cmd *Command, command string, code int) {
	_ = ShowCommandHelp(ctx, cmd, command)
	OsExiter(code)
}

// DefaultShowCommandHelp is the default implementation of ShowCommandHelp.
func DefaultShowCommandHelp(ctx context.Context, cmd *Command, commandName string) error {
	for _, subCmd := range cmd.Commands {
		if !subCmd.HasName(commandName) {
			continue
		}

		tmpl := subCmd.CustomHelpTemplate
		if tmpl == "" {
			if len(subCmd.VisibleCommands()) == 0 {
				tracef("using CommandHelpTemplate")
				tmpl = CommandHelpTemplate
			} else {
				tracef("using SubcommandHelpTemplate")
				tmpl = SubcommandHelpTemplate
			}
		}

		tracef("running HelpPrinter")
		HelpPrinter(cmd.Root().Writer, tmpl, subCmd)

		tracef("returning nil after printing help")
		return nil
	}

	tracef("no matching command found")

	if cmd.CommandNotFound == nil {
		errMsg := fmt.Sprintf("No help topic for '%v'", commandName)

		if cmd.Suggest {
			// Hidden commands must not be suggested, as in shell completion.
			var commands []*Command
			for _, subCmd := range cmd.Commands {
				if !subCmd.Hidden {
					commands = append(commands, subCmd)
				}
			}
			if suggestion := SuggestCommand(commands, commandName); suggestion != "" {
				errMsg += ". " + suggestion
			}
		}

		tracef("exiting 3 with errMsg %[1]q", errMsg)
		return Exit(errMsg, 3)
	}

	tracef("running CommandNotFound func for %[1]q", commandName)
	cmd.CommandNotFound(ctx, cmd, commandName)

	return nil
}

// ShowSubcommandHelpAndExit prints help for the given subcommand via ShowSubcommandHelp and exits with exit code.
func ShowSubcommandHelpAndExit(cmd *Command, exitCode int) {
	_ = ShowSubcommandHelp(cmd)
	OsExiter(exitCode)
}

// DefaultShowSubcommandHelp is the default implementation of ShowSubcommandHelp.
func DefaultShowSubcommandHelp(cmd *Command) error {
	HelpPrinter(cmd.Root().Writer, SubcommandHelpTemplate, cmd)
	return nil
}

// ShowVersion prints the version number of the root Command.
func ShowVersion(cmd *Command) {
	tracef("showing version via VersionPrinter (cmd=%[1]q)", cmd.Name)
	VersionPrinter(cmd)
}

// DefaultPrintVersion is the default implementation of VersionPrinter.
func DefaultPrintVersion(cmd *Command) {
	_, _ = fmt.Fprintf(cmd.Root().Writer, "%v version %v\n", cmd.Name, cmd.Version)
}

func handleTemplateError(err error) {
	if err != nil {
		tracef("error encountered during template parse: %[1]v", err)
		// If the writer is closed, t.Execute will fail, and there's nothing
		// we can do to recover.
		if os.Getenv("CLI_TEMPLATE_ERROR_DEBUG") != "" {
			_, _ = fmt.Fprintf(ErrWriter, "CLI TEMPLATE ERROR: %#v\n", err)
		}
		return
	}
}

// DefaultPrintHelp is the default implementation of HelpPrinter.
func DefaultPrintHelp(out io.Writer, templ string, data any) {
	HelpPrinterCustom(out, templ, data, nil)
}

func checkVersion(cmd *Command) bool {
	return cmd.versionFlag != nil && cmd.versionFlag.IsSet()
}

func subtract(a, b int) int {
	return a - b
}

func indent(spaces int, v string) string {
	pad := strings.Repeat(" ", spaces)
	return pad + strings.ReplaceAll(v, "\n", "\n"+pad)
}

func nindent(spaces int, v string) string {
	return "\n" + indent(spaces, v)
}

func wrap(input string, offset int, wrapAt int) string {
	var ss []string

	lines := strings.Split(input, "\n")

	padding := strings.Repeat(" ", offset)

	for i, line := range lines {
		if line == "" {
			ss = append(ss, line)
		} else {
			wrapped := wrapLine(line, offset, wrapAt, padding)
			if i == 0 {
				ss = append(ss, wrapped)
			} else {
				ss = append(ss, padding+wrapped)
			}

		}
	}

	return strings.Join(ss, "\n")
}

func wrapLine(input string, offset int, wrapAt int, padding string) string {
	lineWidth := wrapAt - offset

	// Like offset, count runes rather than bytes.
	if lineWidth <= 0 || utf8.RuneCountInString(input) <= lineWidth {
		return input
	}

	words := strings.Fields(input)
	if len(words) == 0 {
		return input
	}

	var wrapped strings.Builder
	wrapped.WriteString(words[0])
	spaceLeft := lineWidth - utf8.RuneCountInString(words[0])
	for _, word := range words[1:] {
		width := utf8.RuneCountInString(word)
		if width+1 > spaceLeft {
			wrapped.WriteString("\n" + padding + word)
			spaceLeft = lineWidth - width
		} else {
			wrapped.WriteString(" " + word)
			spaceLeft -= 1 + width
		}
	}

	return wrapped.String()
}

// offset counts runes, not bytes, the same way the tabwriter that aligns
// the help output does.
func offset(input string, fixed int) int {
	return utf8.RuneCountInString(input) + fixed
}

// this function tries to find the max width of the names column
// so say we have the following rows for help
//
//	foo1, foo2, foo3  some string here
//	bar1, b2 some other string here
//
// We want to offset the 2nd row usage by some amount so that everything
// is aligned
//
//	foo1, foo2, foo3  some string here
//	bar1, b2          some other string here
//
// to find that offset we find the length of all the rows and use the max
// to calculate the offset
func offsetCommands(cmds []*Command, fixed int) int {
	max := 0
	for _, cmd := range cmds {
		if n := utf8.RuneCountInString(strings.Join(cmd.Names(), ", ")); n > max {
			max = n
		}
	}
	return max + fixed
}
