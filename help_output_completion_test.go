//go:build !urfave_cli_no_completion

package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// checkHelpOutput appends the fish completion of the test command to got,
// writes the golden file when UPDATE_GOLDEN is set, and compares the whole
// output against the golden file.
func checkHelpOutput(t *testing.T, got *strings.Builder) {
	fish, err := buildHelpOutputTestCommand().ToFishCompletion()
	require.NoError(t, err)
	got.WriteString("=== fish completion\n" + fish)

	if os.Getenv("UPDATE_GOLDEN") != "" {
		require.NoError(t, os.WriteFile(helpOutputFile, []byte(got.String()), 0o644))
	}
	expectFileContent(t, helpOutputFile, got.String())
}
