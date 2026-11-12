package cli

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// reflectMethodSymbols builds examples/example-cli with the given tags,
// and returns the number of reachable symbols using reflect.Method or
// reflect.MethodByName with a non-constant argument. If there are any,
// the linker's dead code elimination of exported methods is disabled.
func reflectMethodSymbols(t *testing.T, tags string) int {
	t.Helper()
	out := filepath.Join(t.TempDir(), "example-cli")
	cmd := exec.Command("go", "build", "-tags", tags, "-ldflags=-dumpdep", "-o", out, "./examples/example-cli")
	deps, err := cmd.CombinedOutput()
	require.NoError(t, err, string(deps))
	return strings.Count(string(deps), "<ReflectMethod>")
}

// Using text/template (or anything else calling reflect.Method or
// reflect.MethodByName with a non-constant argument) disables linker's
// dead code elimination of exported methods. Make sure this does not
// happen with urfave_cli_no_template tag.
func TestNoTemplateTagDCE(t *testing.T) {
	// Sanity check: without the tag, DCE is disabled by text/template.
	assert.NotZero(t, reflectMethodSymbols(t, ""))

	assert.Zero(t, reflectMethodSymbols(t, "urfave_cli_no_template"))
}

// completionSymbols builds examples/example-cli with the given tags, and
// returns the number of reachable symbols that implement shell completion:
// the completion machinery, the embedded shell scripts, and fish completion.
func completionSymbols(t *testing.T, tags string) int {
	t.Helper()
	out := filepath.Join(t.TempDir(), "example-cli")
	cmd := exec.Command("go", "build", "-tags", tags, "-o", out, "./examples/example-cli")
	built, err := cmd.CombinedOutput()
	require.NoError(t, err, string(built))

	nm := exec.Command("go", "tool", "nm", out)
	syms, err := nm.CombinedOutput()
	require.NoError(t, err, string(syms))

	count := 0
	for _, sym := range []string{
		"cli/v3.DefaultCompleteWithFlags",
		"cli/v3.buildCompletionCommand",
		"cli/v3.autoCompleteFS",
		"cli/v3.checkShellCompleteFlag",
		"cli/v3.(*Command).ToFishCompletion",
	} {
		count += strings.Count(string(syms), sym)
	}
	return count
}

// EnableShellCompletion is a runtime option, so without a build tag the
// completion code is reachable and stays in the binary even when completion
// is left disabled (#2291). Make sure the urfave_cli_no_completion tag
// removes it, alone and combined with urfave_cli_no_template.
func TestNoCompletionTagDCE(t *testing.T) {
	// Sanity check: without the tag, the completion code is linked in.
	assert.NotZero(t, completionSymbols(t, ""))

	assert.Zero(t, completionSymbols(t, "urfave_cli_no_completion"))
	assert.Zero(t, completionSymbols(t, "urfave_cli_no_template,urfave_cli_no_completion"))
}
