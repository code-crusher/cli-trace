package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testCatalog is a partial translation used to prove that a catalogue only
// needs the messages it wants to change.
var testCatalog = &Catalog{
	Tag:    "xx",
	Plural: OneOther,
	Messages: map[string]Text{
		"error.unknown_flag": {Other: "unknown flag <{flag}>"},
		"error.required_flag": {
			One:   "missing flag <{flag}>",
			Other: "missing flags <{flag}>",
		},
		"error.usage": {Other: "bad usage: {cause}"},
	},
}

func envLookup(kv ...string) func(string) (string, bool) {
	m := map[string]string{}
	for i := 0; i < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	return func(k string) (string, bool) {
		v, ok := m[k]
		return v, ok
	}
}

func TestCatalogMessage(t *testing.T) {
	tests := []struct {
		name string
		cat  *Catalog
		key  string
		args MessageArgs
		want string
	}{
		{
			name: "translated key",
			cat:  testCatalog,
			key:  "error.unknown_flag",
			args: MessageArgs{Flag: "z"},
			want: "unknown flag <z>",
		},
		{
			name: "plural one",
			cat:  testCatalog,
			key:  "error.required_flag",
			args: MessageArgs{Flag: "a", Count: 1},
			want: "missing flag <a>",
		},
		{
			name: "plural other",
			cat:  testCatalog,
			key:  "error.required_flag",
			args: MessageArgs{Flag: "a, b", Count: 2},
			want: "missing flags <a, b>",
		},
		{
			name: "missing key falls back to English",
			cat:  testCatalog,
			key:  "error.missing_value",
			args: MessageArgs{Flag: "-n"},
			want: "flag needs an argument: -n",
		},
		{
			name: "nil plural uses Other",
			cat:  &Catalog{Tag: "yy", Messages: map[string]Text{"k": {One: "one", Other: "other"}}},
			key:  "k",
			args: MessageArgs{Count: 1},
			want: "other",
		},
		{
			name: "unknown key in English returns the key",
			cat:  English,
			key:  "no.such.key",
			want: "no.such.key",
		},
		{
			name: "explicit fallback wins over English",
			cat: &Catalog{
				Tag:      "zz",
				Messages: map[string]Text{},
				Fallback: &Catalog{Messages: map[string]Text{"k": {Other: "from fallback"}}},
			},
			key:  "k",
			want: "from fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.cat.Message(tt.key, tt.args))
		})
	}
}

func TestTextForm(t *testing.T) {
	text := Text{Zero: "z", One: "o", Other: "x"}

	assert.Equal(t, "z", text.form(Zero))
	assert.Equal(t, "o", text.form(One))
	assert.Equal(t, "x", text.form(Other))
	assert.Equal(t, "x", text.form(Two), "a missing form must fall back to Other")
	assert.Equal(t, "x", text.form(Few), "a missing form must fall back to Other")
	assert.Equal(t, "x", text.form(Many), "a missing form must fall back to Other")
}

func TestExpand(t *testing.T) {
	args := MessageArgs{
		Command:  "app",
		Flag:     "name",
		Arg:      "target",
		Value:    "v",
		Choices:  "a, b",
		Cause:    "boom",
		Count:    3,
		Expected: 5,
	}

	assert.Equal(
		t,
		"app name target v a, b boom 3 5",
		expand("{command} {flag} {arg} {value} {choices} {cause} {count} {expected}", args),
	)
	assert.Equal(t, "plain", expand("plain", args), "a message with no placeholder is returned as is")
}

func TestKeysAreSortedAndCoverEnglish(t *testing.T) {
	keys := Keys()

	assert.True(t, slices.IsSorted(keys))
	assert.Equal(t, len(English.Messages), len(keys))
	assert.Contains(t, keys, "error.unknown_flag")
	assert.Contains(t, keys, "error.required_flag")
}

func TestDetectLocales(t *testing.T) {
	tests := []struct {
		name string
		env  func(string) (string, bool)
		want []string
	}{
		{"nothing set", envLookup(), nil},
		{"LANG", envLookup("LANG", "de_DE.UTF-8"), []string{"de-DE"}},
		{"modifier", envLookup("LANG", "de_DE@euro"), []string{"de-DE"}},
		{"LC_MESSAGES beats LANG", envLookup("LANG", "fr_FR", "LC_MESSAGES", "de_AT"), []string{"de-AT"}},
		{"LC_ALL beats both", envLookup("LANG", "fr_FR", "LC_MESSAGES", "de_AT", "LC_ALL", "pt_BR.UTF-8"), []string{"pt-BR"}},
		{"empty LC_ALL is skipped", envLookup("LC_ALL", "", "LANG", "de_DE"), []string{"de-DE"}},
		{"C means default", envLookup("LANG", "C.UTF-8"), nil},
		{"POSIX means default", envLookup("LC_ALL", "POSIX"), nil},
		{"LANGUAGE list first", envLookup("LANG", "en_US", "LANGUAGE", "de:fr_FR"), []string{"de", "fr-FR", "en-US"}},
		{"LANGUAGE ignored under C", envLookup("LANG", "C", "LANGUAGE", "de"), nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, DetectLocales(tt.env))
		})
	}
}

func TestSelectLocalizer(t *testing.T) {
	deAT := &Catalog{Tag: "xx-AT"}

	tests := []struct {
		name string
		env  func(string) (string, bool)
		cats []*Catalog
		want Localizer
	}{
		{"no locale", envLookup(), []*Catalog{testCatalog}, English},
		{"language match", envLookup("LANG", "xx_CH.UTF-8"), []*Catalog{testCatalog}, testCatalog},
		{"exact beats language", envLookup("LANG", "xx_AT"), []*Catalog{testCatalog, deAT}, deAT},
		{"no match", envLookup("LANG", "ja_JP"), []*Catalog{testCatalog}, English},
		{"LANGUAGE order", envLookup("LANG", "ja_JP", "LANGUAGE", "ja:xx"), []*Catalog{testCatalog}, testCatalog},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, SelectLocalizer(tt.env, tt.cats...))
		})
	}
}

// The library's own errors keep an English Error method, so callers can match
// on their text, but Localize renders them in the command's language.
func TestCommandLocalize(t *testing.T) {
	cmd := &Command{Name: "app", Localizer: testCatalog}

	tests := []struct {
		name string
		err  error
		want string
	}{
		{"unknown flag", newErrUnknownFlag("z"), "unknown flag <z>"},
		{"required flag", &errRequiredFlags{missingFlags: []string{"a"}}, "missing flag <a>"},
		{"required flags", &errRequiredFlags{missingFlags: []string{"a", "b"}}, "missing flags <a, b>"},
		{"wrapped ExitCoder", Exit(newErrUnknownFlag("z"), 2), "unknown flag <z>"},
		{"foreign error is unchanged", errors.New("boom"), "boom"},
		{"nil error", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, cmd.Localize(tt.err))
		})
	}

	// The Error method itself stays English.
	assert.Equal(t, "flag provided but not defined: -z", newErrUnknownFlag("z").Error())
}

func TestCommandLocalizerInherited(t *testing.T) {
	root := &Command{Name: "root", Localizer: testCatalog}
	sub := &Command{Name: "sub", parent: root}

	assert.Equal(t, testCatalog, sub.localizer(), "a subcommand must inherit the root Localizer")

	sub.Localizer = English
	assert.Equal(t, English, sub.localizer(), "a subcommand's own Localizer wins")
	assert.Equal(t, testCatalog, root.localizer(), "the root is unaffected")
}

func TestDefaultLocalizer(t *testing.T) {
	old := DefaultLocalizer
	t.Cleanup(func() { DefaultLocalizer = old })

	DefaultLocalizer = testCatalog
	assert.Equal(t, testCatalog, (&Command{}).localizer())

	DefaultLocalizer = nil
	assert.Equal(t, English, (&Command{}).localizer(), "a nil DefaultLocalizer means English")
}

// A translated catalogue changes what the library prints, while the error
// value a caller receives still reports English text.
func TestRunPrintsLocalizedUsageError(t *testing.T) {
	var errOut bytes.Buffer
	cmd := &Command{
		Name:      "app",
		Writer:    io.Discard,
		ErrWriter: &errOut,
		Localizer: testCatalog,
	}

	err := cmd.Run(context.Background(), []string{"app", "--nope"})

	require.Error(t, err)
	assert.Contains(t, errOut.String(), "bad usage: unknown flag <nope>")
	assert.Contains(t, err.Error(), "flag provided but not defined: -nope")
}

func TestRunPrintsLocalizedRequiredFlagError(t *testing.T) {
	var errOut bytes.Buffer
	cmd := &Command{
		Name:      "app",
		Writer:    io.Discard,
		ErrWriter: &errOut,
		Localizer: testCatalog,
		Flags:     []Flag{&StringFlag{Name: "name", Required: true}},
		Action:    func(context.Context, *Command) error { return nil },
	}

	err := cmd.Run(context.Background(), []string{"app"})

	require.Error(t, err)
	assert.Contains(t, errOut.String(), "bad usage: missing flag <name>")
	assert.Equal(t, `Required flag "name" not set`, err.Error())
}

// An error returned by an action is printed by HandleExitCoder, which renders
// it with the command's Localizer.
func TestRunPrintsLocalizedActionError(t *testing.T) {
	cmd := &Command{
		Name:      "app",
		Writer:    io.Discard,
		ErrWriter: io.Discard,
		Localizer: testCatalog,
		Action: func(context.Context, *Command) error {
			return Exit(newErrUnknownFlag("boom"), 7)
		},
	}

	code, called, out := captureExitWith(t, cmd)

	assert.True(t, called)
	assert.Equal(t, 7, code)
	assert.Contains(t, out, "unknown flag <boom>")
}

// captureExitWith runs cmd with the package-level OsExiter and ErrWriter
// replaced, like captureExit, and reports what HandleExitCoder saw.
func captureExitWith(t *testing.T, cmd *Command) (int, bool, string) {
	t.Helper()

	exitCode := 0
	called := false
	OsExiter = func(rc int) {
		if !called {
			exitCode = rc
			called = true
		}
	}
	t.Cleanup(func() { OsExiter = fakeOsExiter })

	var buf bytes.Buffer
	oldWriter := ErrWriter
	ErrWriter = &buf
	t.Cleanup(func() { ErrWriter = oldWriter })

	_ = cmd.Run(context.Background(), []string{"app"})

	return exitCode, called, buf.String()
}

// Deprecation warnings go through the catalogue too.
func TestLocalizedDeprecationWarnings(t *testing.T) {
	var errOut bytes.Buffer
	cmd := &Command{
		Name:       "app",
		Writer:     io.Discard,
		ErrWriter:  &errOut,
		Localizer:  testCatalog,
		Deprecated: "use new-app",
		Action:     func(context.Context, *Command) error { return nil },
	}

	require.NoError(t, cmd.Run(context.Background(), []string{"app"}))
	// testCatalog does not translate the warning, so English is used.
	assert.Contains(t, errOut.String(), `Command "app" is deprecated, use new-app`)

}

// Every library error type that implements localizable must have a catalogue
// entry, otherwise a translation silently falls back to English.
func TestLocalizableErrorsUseCatalogueKeys(t *testing.T) {
	errs := []error{
		newErrUnknownFlag("z"),
		newErrMissingValue("-n"),
		&errInvalidFlagValue{value: "v", flag: "f", err: errors.New("cause")},
		&errInvalidArgValue{value: "v", arg: "a", err: errors.New("cause")},
		&errRequiredFlags{missingFlags: []string{"f"}},
		&errRequiredArguments{missingArguments: []string{"a"}},
		&errArgsCount{arg: "a", count: 0, expected: 1},
		&mutuallyExclusiveGroup{flag1Name: "a", flag2Name: "b"},
		&mutuallyExclusiveGroupRequiredFlag{flags: &MutuallyExclusiveFlags{
			Flags: [][]Flag{{&BoolFlag{Name: "a"}}},
		}},
		&errBothSetFlags{flag: "--a", inverse: "--no-a"},
		&errDuplicateFlag{},
		&errSourceValue{value: "v", typ: "int", source: "env VAR", flag: "f", err: errors.New("cause")},
		&errNoHelpTopic{topic: "x"},
		&errNoHelpTopic{topic: "x", suggestion: "y"},
	}

	for _, err := range errs {
		le, ok := err.(localizable)
		require.True(t, ok, "%T does not implement localizable", err)
		assert.NotEmpty(t, le.localize(English), "%T has no English message", err)
	}
}

// A catalogue that translates nothing must still produce every message.
func TestEmptyCatalogFallsBackForEveryError(t *testing.T) {
	empty := &Catalog{Tag: "empty", Messages: map[string]Text{}}
	cmd := &Command{Name: "app", Localizer: empty}

	for _, err := range []error{
		newErrUnknownFlag("z"),
		&errRequiredFlags{missingFlags: []string{"f"}},
		&errRequiredArguments{missingArguments: []string{"a"}},
	} {
		assert.NotEmpty(t, cmd.Localize(err))
	}
}

func TestUsageErrorMessageIsLocalized(t *testing.T) {
	cmd := &Command{Name: "app", Localizer: testCatalog}
	assert.Equal(t, "bad usage: boom", cmd.usageErrorText(errors.New("boom")))
	assert.Equal(t, "bad usage: unknown flag <z>", cmd.usageErrorText(newErrUnknownFlag("z")))
	assert.True(t, strings.HasPrefix(cmd.Message("error.usage", MessageArgs{Cause: "x"}), "bad usage:"))
}
