package cli

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"unicode/utf8"
)

func prefixFor(name string) (prefix string) {
	if utf8.RuneCountInString(name) == 1 {
		prefix = "-"
	} else {
		prefix = "--"
	}

	return prefix
}

// Returns the placeholder, if any, and the unquoted usage string.
func unquoteUsage(usage string) (string, string) {
	for i := 0; i < len(usage); i++ {
		if usage[i] == '`' {
			for j := i + 1; j < len(usage); j++ {
				if usage[j] == '`' {
					name := usage[i+1 : j]
					usage = usage[:i] + name + usage[j+1:]
					return name, usage
				}
			}
			break
		}
	}
	return "", usage
}

func prefixedNames(names []string, placeholder string) string {
	var prefixed strings.Builder
	for i, name := range names {
		if name == "" {
			continue
		}

		prefixed.WriteString(prefixFor(name) + name)
		if placeholder != "" {
			prefixed.WriteString(" " + placeholder)
		}
		if i < len(names)-1 {
			prefixed.WriteString(", ")
		}
	}
	return prefixed.String()
}

func envFormat(envVars []string, prefix, sep, suffix string) string {
	if len(envVars) > 0 {
		return fmt.Sprintf(" [%s%s%s]", prefix, strings.Join(envVars, sep), suffix)
	}
	return ""
}

func defaultEnvFormat(envVars []string) string {
	return envFormat(envVars, "$", ", $", "")
}

func withEnvHint(envVars []string, str string) string {
	envText := ""
	if runtime.GOOS != "windows" || os.Getenv("PSHOME") != "" {
		envText = defaultEnvFormat(envVars)
	} else {
		envText = envFormat(envVars, "%", "%, %", "%")
	}
	return str + envText
}

func withFileHint(filePath, str string) string {
	fileText := ""
	if filePath != "" {
		fileText = fmt.Sprintf(" [%s]", filePath)
	}
	return str + fileText
}

func formatDefault(format string) string {
	return " (default: " + format + ")"
}

// stringifyFlag renders a flag as the help output's two tab-delimited
// columns: the prefixed names, e.g. "--lang value, -l value", and the usage
// line including default value and environment variable hints.
func stringifyFlag(f Flag) string {
	// enforce DocGeneration interface on flags to avoid reflection
	df, ok := f.(DocGenerationFlag)
	if !ok {
		return ""
	}
	nameColumn, usageColumn := stringifyFlagColumnsDF(f, df)
	return FlagEnvHinter(df.GetEnvVars(), fmt.Sprintf("%s\t%s", nameColumn, usageColumn))
}

// flagNamesAndPlaceholder computes the first help column of flag f: its
// prefixed names and the value placeholder, if any. It returns false if f
// does not implement [DocGenerationFlag], i.e. is not a flag whose help
// output can be generated.
func flagNamesAndPlaceholder(f Flag) (string, bool) {
	// enforce DocGeneration interface on flags to avoid reflection
	df, ok := f.(DocGenerationFlag)
	if !ok {
		return "", false
	}
	return flagNamesAndPlaceholderDF(f, df), true
}

// flagNamesAndPlaceholderDF is flagNamesAndPlaceholder with the flag's
// DocGenerationFlag interface already asserted by the caller.
func flagNamesAndPlaceholderDF(f Flag, df DocGenerationFlag) string {
	placeholder, _ := unquoteUsage(df.GetUsage())
	needsPlaceholder := df.TakesValue()
	// if needsPlaceholder is true, placeholder is empty
	if needsPlaceholder && placeholder == "" {
		// try to get type from flag
		if tname := df.TypeName(); tname != "" {
			placeholder = tname
		} else {
			placeholder = defaultPlaceholder
		}
	}

	pn := FlagNamePrefixer(f.Names(), placeholder)
	sliceFlag, ok := f.(DocGenerationMultiValueFlag)
	if ok && sliceFlag.IsMultiValueFlag() {
		pn = pn + " [ " + pn + " ]"
	}

	return pn
}

// stringifyFlagColumns splits the help output of flag f into its two
// tab-delimited columns. The first is the prefixed flag names and value
// placeholder, computed by [flagNamesAndPlaceholder]. The second is the usage
// line with the default value appended, e.g. "greeting to use (default:
// hello)". The environment variable hint is not part of either column; it
// is appended by [stringifyFlag].
//
// It returns ok=false if f does not implement [DocGenerationFlag].
func stringifyFlagColumns(f Flag) (nameColumn, usageColumn string, ok bool) {
	// enforce DocGeneration interface on flags to avoid reflection
	df, ok := f.(DocGenerationFlag)
	if !ok {
		return "", "", false
	}
	nameColumn, usageColumn = stringifyFlagColumnsDF(f, df)
	return nameColumn, usageColumn, true
}

// stringifyFlagColumnsDF is stringifyFlagColumns with the flag's
// DocGenerationFlag interface already asserted by the caller.
func stringifyFlagColumnsDF(f Flag, df DocGenerationFlag) (nameColumn, usageColumn string) {
	_, usage := unquoteUsage(df.GetUsage())

	defaultValueString := ""

	// don't print default text for required flags
	if rf, ok := f.(RequiredFlag); !ok || !rf.IsRequired() {
		if df.IsDefaultVisible() {
			if s := df.GetDefaultText(); s != "" {
				defaultValueString = fmt.Sprintf(formatDefault("%s"), s)
			} else if df.TakesValue() && df.GetValue() != "" {
				defaultValueString = fmt.Sprintf(formatDefault("%s"), df.GetValue())
			}
		}
	}

	return flagNamesAndPlaceholderDF(f, df), strings.TrimSpace(usage + defaultValueString)
}
