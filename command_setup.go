package cli

import (
	"flag"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

func (cmd *Command) setupDefaults(osArgs []string) {
	if cmd.didSetupDefaults {
		tracef("already did setup (cmd=%[1]q)", cmd.Name)
		return
	}

	cmd.didSetupDefaults = true

	isRoot := cmd.parent == nil
	tracef("isRoot? %[1]v (cmd=%[2]q)", isRoot, cmd.Name)

	if cmd.ShellComplete == nil {
		tracef("setting default ShellComplete (cmd=%[1]q)", cmd.Name)
		cmd.ShellComplete = DefaultCompleteWithFlags
	}

	if cmd.Name == "" && isRoot && len(osArgs) > 0 {
		name := filepath.Base(osArgs[0])
		tracef("setting cmd.Name from first arg basename (cmd=%[1]q)", name)
		cmd.Name = name
	}

	if cmd.Usage == "" && isRoot {
		tracef("setting default Usage (cmd=%[1]q)", cmd.Name)
		cmd.Usage = "A new cli application"
	}

	if cmd.Version == "" {
		tracef("setting HideVersion=true due to empty Version (cmd=%[1]q)", cmd.Name)
		cmd.HideVersion = true
	}

	if cmd.Action == nil {
		tracef("setting default Action as help command action (cmd=%[1]q)", cmd.Name)
		cmd.Action = helpCommandAction
	}

	if cmd.Reader == nil {
		if cmd.parent != nil && cmd.parent.Reader != nil {
			tracef("inheriting Reader from parent (cmd=%[1]q)", cmd.Name)
			cmd.Reader = cmd.parent.Reader
		} else {
			tracef("setting default Reader as os.Stdin (cmd=%[1]q)", cmd.Name)
			cmd.Reader = os.Stdin
		}
	}

	if cmd.Writer == nil {
		if cmd.parent != nil && cmd.parent.Writer != nil {
			tracef("inheriting Writer from parent (cmd=%[1]q)", cmd.Name)
			cmd.Writer = cmd.parent.Writer
		} else {
			tracef("setting default Writer as os.Stdout (cmd=%[1]q)", cmd.Name)
			cmd.Writer = os.Stdout
		}
	}

	if cmd.ErrWriter == nil {
		if cmd.parent != nil && cmd.parent.ErrWriter != nil {
			tracef("inheriting ErrWriter from parent (cmd=%[1]q)", cmd.Name)
			cmd.ErrWriter = cmd.parent.ErrWriter
		} else {
			tracef("setting default ErrWriter as os.Stderr (cmd=%[1]q)", cmd.Name)
			cmd.ErrWriter = os.Stderr
		}
	}

	if cmd.AllowExtFlags {
		tracef("visiting all flags given AllowExtFlags=true (cmd=%[1]q)", cmd.Name)
		// add global flags added by other packages
		flag.VisitAll(func(f *flag.Flag) {
			// skip test flags, and flags whose name a flag of this command
			// or a persistent flag of an ancestor already has, as that flag
			// wins. An ancestor's Go flags are persistent too.
			if !strings.HasPrefix(f.Name, ignoreFlagPrefix) &&
				!flagNamesInUse(cmd.allFlags(), []string{f.Name}) &&
				!cmd.hasPersistentFlagNameOnAncestor(f.Name) {
				cmd.Flags = append(cmd.Flags, &extFlag{f: f, set: flag.CommandLine.Set})
			}
		})
	}

	for _, subCmd := range cmd.Commands {
		tracef("setting sub-command (cmd=%[1]q) parent as self (cmd=%[2]q)", subCmd.Name, cmd.Name)
		subCmd.parent = cmd
	}

	cmd.ensureHelp()

	if !cmd.HideVersion && isRoot {
		tracef("appending version flag (cmd=%[1]q)", cmd.Name)
		if !cmd.globaVersionFlagAdded {
			var localVersionFlag Flag
			if globalVersionFlag, ok := VersionFlag.(*BoolFlag); ok {
				flag := *globalVersionFlag
				// Drop any alias a user flag already claims (e.g. -v
				// for --verbose) so the user flag wins but --version
				// still works. See #2229.
				flag.Aliases = dropClashingAliases(flag.Aliases, cmd.allFlags(), flag.Name)
				localVersionFlag = &flag
			} else {
				localVersionFlag = VersionFlag
			}

			if !flagNamesInUse(cmd.allFlags(), localVersionFlag.Names()) {
				cmd.appendFlag(localVersionFlag)
				cmd.versionFlag = localVersionFlag
				cmd.globaVersionFlagAdded = true
			}
		}
	}

	if cmd.PrefixMatchCommands && cmd.SuggestCommandFunc == nil {
		tracef("setting default SuggestCommandFunc (cmd=%[1]q)", cmd.Name)
		cmd.SuggestCommandFunc = suggestCommand
	}

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

	tracef("setting command categories (cmd=%[1]q)", cmd.Name)
	cmd.categories = newCommandCategories()

	for _, subCmd := range cmd.Commands {
		cmd.categories.AddCommand(subCmd.Category, subCmd)
	}

	tracef("sorting command categories (cmd=%[1]q)", cmd.Name)
	sort.Sort(cmd.categories.(*commandCategories))

	tracef("setting category on mutually exclusive flags (cmd=%[1]q)", cmd.Name)
	for _, grp := range cmd.MutuallyExclusiveFlags {
		grp.propagateCategory()
		grp.propagateStringer()
	}

	tracef("setting flag categories (cmd=%[1]q)", cmd.Name)
	cmd.flagCategories = newFlagCategoriesFromFlags(cmd.allFlags())

	if cmd.Metadata == nil {
		tracef("setting default Metadata (cmd=%[1]q)", cmd.Name)
		cmd.Metadata = map[string]any{}
	}

	cmd.setFlags = map[Flag]struct{}{}
}

func (cmd *Command) setupCommandGraph() {
	tracef("setting up command graph (cmd=%[1]q)", cmd.Name)

	_ = cmd.Walk(func(sub *Command) error {
		for _, subCmd := range sub.Commands {
			subCmd.parent = sub
			subCmd.setupSubcommand()
		}
		return nil
	})
}

func (cmd *Command) setupSubcommand() {
	tracef("setting up self as sub-command (cmd=%[1]q)", cmd.Name)

	cmd.ensureHelp()

	tracef("setting command categories (cmd=%[1]q)", cmd.Name)
	cmd.categories = newCommandCategories()

	for _, subCmd := range cmd.Commands {
		cmd.categories.AddCommand(subCmd.Category, subCmd)
	}

	tracef("sorting command categories (cmd=%[1]q)", cmd.Name)
	sort.Sort(cmd.categories.(*commandCategories))

	tracef("setting category on mutually exclusive flags (cmd=%[1]q)", cmd.Name)
	for _, grp := range cmd.MutuallyExclusiveFlags {
		grp.propagateCategory()
		grp.propagateStringer()
	}

	tracef("setting flag categories (cmd=%[1]q)", cmd.Name)
	cmd.flagCategories = newFlagCategoriesFromFlags(cmd.allFlags())
}

func flagNamesInUse(flags []Flag, names []string) bool {
	for _, name := range names {
		for _, fl := range flags {
			if slices.Contains(fl.Names(), name) {
				return true
			}
		}
	}

	return false
}

func (cmd *Command) hideHelp() bool {
	tracef("hide help (cmd=%[1]q)", cmd.Name)
	for c := cmd; c != nil; c = c.parent {
		if c.HideHelp {
			return true
		}
	}

	return false
}

func (cmd *Command) hideHelpCommand() bool {
	tracef("hide help command (cmd=%[1]q)", cmd.Name)
	for c := cmd; c != nil; c = c.parent {
		if c.HideHelpCommand {
			return true
		}
	}

	return false
}

func (cmd *Command) ensureHelp() {
	tracef("ensuring help (cmd=%[1]q)", cmd.Name)

	helpCommand := buildHelpCommand(true)

	hideHelp := cmd.hideHelp()
	if !hideHelp {
		if cmd.Command(helpCommand.Name) == nil {
			if !cmd.hideHelpCommand() {
				// Drop any alias a user command already claims (e.g. h
				// for history) so the user command wins and help does not
				// list it, as for the help and version flags.
				helpCommand.Aliases = slices.DeleteFunc(helpCommand.Aliases, func(alias string) bool {
					return cmd.Command(alias) != nil
				})
				tracef("appending helpCommand (cmd=%[1]q)", cmd.Name)
				cmd.appendCommand(helpCommand)
			}
		}
	}
	cmd.ensureHelpFlag(!hideHelp)
}

// ensureHelpFlag resolves the flag that triggers help for cmd and records
// it on cmd.helpFlag. Reading the global HelpFlag exactly once here keeps
// concurrent runs from observing a half-initialized flag and keeps the
// other code paths from touching the global.
//
// A *BoolFlag global is cloned per command so parallel runs don't share
// mutable flag state. Aliases already claimed by a user flag are dropped
// so the user flag wins. If no name of the help flag is free, or the
// global is a different Flag implementation, the user flag that took the
// primary name still triggers help. When appendBuiltIn is false (hideHelp
// is set) the built-in flag is not added; a user flag that took the help
// flag's name still triggers help. If HelpFlag is nil, cmd.helpFlag
// stays nil and no help flag is recognized.
func (cmd *Command) ensureHelpFlag(appendBuiltIn bool) {
	if cmd.helpFlag != nil {
		// already resolved (ensureHelp may run more than once for the
		// same command); don't add the built-in flag twice
		return
	}
	helpFlag := HelpFlag
	if helpFlag == nil {
		return
	}
	candidate := helpFlag
	if globalHelpFlag, ok := helpFlag.(*BoolFlag); ok {
		flag := *globalHelpFlag
		// Drop any alias a user flag already claims (e.g. -h
		// for --host) so the user flag wins but --help still
		// works, as for the version flag.
		flag.Aliases = dropClashingAliases(flag.Aliases, cmd.allFlags(), flag.Name)
		candidate = &flag
	}
	if appendBuiltIn && !flagNamesInUse(cmd.allFlags(), candidate.Names()) {
		tracef("appending HelpFlag (cmd=%[1]q)", cmd.Name)
		cmd.appendFlag(candidate)
		cmd.helpFlag = candidate
		return
	}
	// The built-in flag is not appended: help is hidden, or every name
	// of the help flag is taken by a user flag. In both cases a user
	// flag that took the help flag's name still triggers help.
	cmd.helpFlag = cmd.lFlag(helpFlag.Names()[0])
}

// dropClashingAliases removes aliases from `aliases` that are already
// claimed by a flag in `userFlags` (either as a primary name or as one
// of its own aliases). Aliases equal to `selfName` are kept so the
// flag's primary name doesn't accidentally remove itself.
func dropClashingAliases(aliases []string, userFlags []Flag, selfName string) []string {
	if len(aliases) == 0 || len(userFlags) == 0 {
		return aliases
	}
	taken := map[string]struct{}{}
	for _, f := range userFlags {
		for _, n := range f.Names() {
			taken[n] = struct{}{}
		}
	}
	kept := aliases[:0:0]
	for _, a := range aliases {
		if a == selfName {
			kept = append(kept, a)
			continue
		}
		if _, ok := taken[a]; ok {
			continue
		}
		kept = append(kept, a)
	}
	return kept
}
