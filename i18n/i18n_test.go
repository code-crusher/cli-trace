package i18n_test

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/urfave/cli/v3"
	"github.com/urfave/cli/v3/i18n"
	"github.com/urfave/cli/v3/i18n/locale/de"
)

func TestLoad(t *testing.T) {
	fsys := fstest.MapFS{"fr.json": {Data: []byte(`{
		"error.unknown_flag": "option inconnue : {flag}",
		"error.required_flag": {"one": "option obligatoire manquante : {flag}", "other": "options obligatoires manquantes : {flag}"}
	}`)}}

	// French treats 0 and 1 as singular.
	plural := func(n int) cli.Form {
		if n == 0 || n == 1 {
			return cli.One
		}
		return cli.Other
	}

	c, err := i18n.Load(fsys, "fr.json", "fr", plural)
	require.NoError(t, err)

	tests := []struct {
		key  string
		args cli.MessageArgs
		want string
	}{
		{"error.unknown_flag", cli.MessageArgs{Flag: "z"}, "option inconnue : z"},
		{"error.required_flag", cli.MessageArgs{Flag: "a", Count: 1}, "option obligatoire manquante : a"},
		{"error.required_flag", cli.MessageArgs{Flag: "a", Count: 4}, "options obligatoires manquantes : a"},
		{"error.missing_value", cli.MessageArgs{Flag: "-n"}, "flag needs an argument: -n"}, // English fallback
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, c.Message(tt.key, tt.args), tt.key)
	}
}

func TestParseErrors(t *testing.T) {
	for _, data := range []string{
		`not json`,
		`{"k": {"one": "x"}}`,
		`{"k": 3}`,
	} {
		_, err := i18n.Parse("xx", nil, []byte(data))
		assert.Error(t, err, "Parse(%s)", data)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := i18n.Load(fstest.MapFS{}, "missing.json", "xx", nil)
	assert.Error(t, err)
}

func TestMissing(t *testing.T) {
	partial := &cli.Catalog{Tag: "xx", Messages: map[string]cli.Text{"error.unknown_flag": {Other: "x"}}}
	missing := i18n.Missing(partial)

	assert.Contains(t, missing, "error.required_flag")
	assert.NotContains(t, missing, "error.unknown_flag")
	assert.Empty(t, i18n.Missing(de.Catalog), "the shipped German catalogue must be complete")
}

func TestParseSingularString(t *testing.T) {
	c, err := i18n.Parse("xx", nil, []byte(`{"k": "plain"}`))
	require.NoError(t, err)

	assert.Equal(t, "plain", c.Message("k", cli.MessageArgs{}))
	assert.Equal(t, cli.Other, cli.Form(0), "a nil plural rule uses Other")
}
