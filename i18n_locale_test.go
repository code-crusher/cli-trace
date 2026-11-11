package cli_test

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cli "github.com/urfave/cli/v3"
	"github.com/urfave/cli/v3/i18n/locale/de"
)

// A shipped catalogue must translate every key, or a user sees a mix of
// languages.
func TestGermanCatalogIsComplete(t *testing.T) {
	for _, key := range cli.Keys() {
		msg, ok := de.Catalog.Messages[key]
		require.True(t, ok, "German catalogue is missing %q", key)
		assert.NotEmpty(t, msg.Other, "German catalogue has no message for %q", key)
	}
}

func TestGermanCatalogEndToEnd(t *testing.T) {
	var errOut bytes.Buffer
	cmd := &cli.Command{
		Name:      "app",
		Writer:    io.Discard,
		ErrWriter: &errOut,
		Localizer: de.Catalog,
	}

	require.Error(t, cmd.Run(context.Background(), []string{"app", "--nope"}))
	assert.Contains(t, errOut.String(), "Falsche Verwendung: Flagge angegeben, aber nicht definiert: -nope")
}

func TestGermanCatalogTranslatesRequiredFlags(t *testing.T) {
	var errOut bytes.Buffer
	cmd := &cli.Command{
		Name:      "app",
		Writer:    io.Discard,
		ErrWriter: &errOut,
		Localizer: de.Catalog,
		Flags:     []cli.Flag{&cli.StringFlag{Name: "name", Required: true}},
		Action:    func(context.Context, *cli.Command) error { return nil },
	}

	require.Error(t, cmd.Run(context.Background(), []string{"app"}))
	assert.Contains(t, errOut.String(), `Falsche Verwendung: Erforderliche Flagge "name" nicht gesetzt`)
}

func TestGermanCatalogTranslatesDeprecationWarnings(t *testing.T) {
	var errOut bytes.Buffer
	cmd := &cli.Command{
		Name:       "app",
		Writer:     io.Discard,
		ErrWriter:  &errOut,
		Localizer:  de.Catalog,
		Deprecated: "benutze new-app",
		Action:     func(context.Context, *cli.Command) error { return nil },
	}

	require.NoError(t, cmd.Run(context.Background(), []string{"app"}))
	assert.Contains(t, errOut.String(), `Befehl "app" ist veraltet, benutze new-app`)
}

// SelectLocalizer picks the catalogue matching the user's environment.
func TestSelectLocalizerFromEnvironment(t *testing.T) {
	env := func(m map[string]string) func(string) (string, bool) {
		return func(k string) (string, bool) {
			v, ok := m[k]
			return v, ok
		}
	}

	tests := []struct {
		name string
		env  map[string]string
		want cli.Localizer
	}{
		{"no locale", nil, cli.English},
		{"German", map[string]string{"LANG": "de_DE.UTF-8"}, de.Catalog},
		{"German without region", map[string]string{"LANG": "de"}, de.Catalog},
		{"unsupported language", map[string]string{"LANG": "ja_JP.UTF-8"}, cli.English},
		{"LANGUAGE overrides LANG", map[string]string{"LANG": "ja_JP", "LANGUAGE": "de"}, de.Catalog},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, cli.SelectLocalizer(env(tt.env), de.Catalog))
		})
	}
}

func ExampleSelectLocalizer() {
	// A command that reports its errors in the user's language, when a
	// catalogue is available for it.
	cmd := &cli.Command{
		Name:      "greet",
		Localizer: cli.SelectLocalizer(func(string) (string, bool) { return "de_DE.UTF-8", true }, de.Catalog),
	}
	_ = cmd

	// Output:
}
