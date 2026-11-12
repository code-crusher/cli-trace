package cli

import (
	"bytes"
	"context"
	"flag"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestIssue2016 checks that a custom help template can format a flag's name
// column and usage column separately, as requested in issue #2016: the flag
// names with their prefixed aliases and value placeholder in one column, and
// the usage text in another.
func TestIssue2016(t *testing.T) {
	skipIfNoTemplates(t)

	cmd := &Command{
		Name:  "app",
		Usage: "sample program",
		Flags: []Flag{
			&StringFlag{
				Name:    "lang",
				Aliases: []string{"l"},
				Usage:   "language for greetings",
			},
			&BoolFlag{Name: "debug", Usage: "enable debug output"},
		},
		CustomRootCommandHelpTemplate: `NAME:
   {{.FullName}} - {{.Usage}}

OPTIONS:
{{range .VisibleFlags}}   {{.NamesWithPlaceholder}}
       {{.GetUsage}}
{{end}}`,
	}

	out := &bytes.Buffer{}
	cmd.Writer = out

	err := cmd.Run(context.Background(), []string{"app", "--help"})
	require.NoError(t, err)

	require.Contains(t, out.String(), `OPTIONS:
   --lang string, -l string
       language for greetings
   --debug
       enable debug output
   --help, -h
       show help
`)
}

// TestIssue2016BoolWithInverse checks the name column of a
// BoolWithInverseFlag, whose help names are the inverse pair, and which
// builds its names independently of FlagStringer.
func TestIssue2016BoolWithInverse(t *testing.T) {
	bif := &BoolWithInverseFlag{
		Name:    "env",
		Aliases: []string{"e"},
		Usage:   "use environment",
	}

	require.Equal(t, "--[no-]env, -e", bif.NamesWithPlaceholder())

	// A FlagStringer whose output has no tab column must not bleed into
	// the names column.
	oldStringer := FlagStringer
	defer func() { FlagStringer = oldStringer }()
	FlagStringer = func(f Flag) string { return "notab" }

	require.Equal(t, "--[no-]env, -e", bif.NamesWithPlaceholder())
}

// TestIssue2016ExtFlag checks the name column of a flag adopted from the Go
// flag package via AllowExtFlags.
func TestIssue2016ExtFlag(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("output", "", "output `FILE`")
	e := &extFlag{f: fs.Lookup("output"), set: fs.Set}

	nameColumn, usageColumn, ok := stringifyFlagColumns(e)
	require.True(t, ok)
	require.Equal(t, "--output FILE", nameColumn)
	require.Equal(t, "output FILE", usageColumn)
	require.Equal(t, "--output FILE", e.NamesWithPlaceholder())
}

// TestIssue2016Columns checks the column split of the flag types based on
// FlagBase, covering placeholder, default value and slice rendering.
func TestIssue2016Columns(t *testing.T) {
	testCases := []struct {
		name        string
		flag        Flag
		nameColumn  string
		usageColumn string
	}{
		{
			name:        "bool flag",
			flag:        &BoolFlag{Name: "debug", Usage: "enable debug output"},
			nameColumn:  "--debug",
			usageColumn: "enable debug output",
		},
		{
			name:        "string flag with placeholder and alias",
			flag:        &StringFlag{Name: "lang", Aliases: []string{"l"}, Usage: "language for greetings"},
			nameColumn:  "--lang string, -l string",
			usageColumn: "language for greetings",
		},
		{
			name:        "string flag with default",
			flag:        &StringFlag{Name: "greeting", Usage: "greeting to use", Value: "hello"},
			nameColumn:  "--greeting string",
			usageColumn: "greeting to use (default: \"hello\")",
		},
		{
			name:        "required flag has no default",
			flag:        &StringFlag{Name: "token", Usage: "auth token", Required: true},
			nameColumn:  "--token string",
			usageColumn: "auth token",
		},
		{
			name:        "usage keeps backquoted placeholder unquoted",
			flag:        &StringFlag{Name: "config", Usage: "config `FILE`"},
			nameColumn:  "--config FILE",
			usageColumn: "config FILE",
		},
		{
			name:        "slice flag",
			flag:        &StringSliceFlag{Name: "items", Usage: "items to use"},
			nameColumn:  "--items string [ --items string ]",
			usageColumn: "items to use",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nameColumn, usageColumn, ok := stringifyFlagColumns(tc.flag)
			require.True(t, ok)
			require.Equal(t, tc.nameColumn, nameColumn)
			require.Equal(t, tc.usageColumn, usageColumn)

			// The columns joined with a tab must reproduce the default
			// flag string, so the split cannot drift from stringifyFlag.
			require.Equal(t, tc.nameColumn+"\t"+tc.usageColumn, stringifyFlag(tc.flag))
		})
	}
}

// TestIssue2016ColumnsWithEnvHint checks that the environment variable hint
// is appended to the usage column and stays out of the names column.
func TestIssue2016ColumnsWithEnvHint(t *testing.T) {
	fl := &StringFlag{Name: "token", Usage: "auth token", Sources: EnvVars("APP_TOKEN")}

	nameColumn, usageColumn, ok := stringifyFlagColumns(fl)
	require.True(t, ok)
	require.Equal(t, "--token string", nameColumn)
	require.Equal(t, "auth token", usageColumn)

	// stringifyFlag appends the env hint, so it must only ever extend the
	// usage column.
	require.Equal(t, "--token string\tauth token [$APP_TOKEN]", stringifyFlag(fl))
}

// TestIssue2016ColumnsNotDocGeneration checks the behavior on a Flag that
// does not implement DocGenerationFlag.
func TestIssue2016ColumnsNotDocGeneration(t *testing.T) {
	_, _, ok := stringifyFlagColumns(Flag(nil))
	require.False(t, ok)
}
