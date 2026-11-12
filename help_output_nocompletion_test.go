//go:build urfave_cli_no_completion

package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// checkHelpOutput writes the golden file when UPDATE_GOLDEN is set and
// compares the help output against the golden file. Built with the
// urfave_cli_no_completion tag there is no fish completion, so the golden
// file's fish section is skipped: the single golden file is shared by all
// build tag combinations.
func checkHelpOutput(t *testing.T, got *strings.Builder) {
	if os.Getenv("UPDATE_GOLDEN") != "" {
		// Writing here would truncate the fish completion section of the
		// shared golden file, so refuse instead.
		require.Empty(t, os.Getenv("UPDATE_GOLDEN"),
			"UPDATE_GOLDEN is not supported with the urfave_cli_no_completion tag")
	}

	data, err := os.ReadFile(helpOutputFile)
	require.NoError(t, err)
	expected := string(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")))
	const marker = "=== fish completion"
	idx := strings.Index(expected, marker)
	require.NotEqual(t, -1, idx, "golden file must contain a fish completion section")
	require.Equal(t, expected[:idx], got.String())
}
