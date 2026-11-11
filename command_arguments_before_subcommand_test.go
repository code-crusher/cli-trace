package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nestedResourceCommand builds the tree from #2067:
//
//	manage author <authorID> book <bookID> delete
//
// Each command's declared argument comes before the subcommand that follows
// it.
func nestedResourceCommand(action ActionFunc) *Command {
	return &Command{
		Name: "manage",
		Commands: []*Command{
			{
				Name: "author",
				Arguments: []Argument{
					&StringArg{Name: "authorID"},
				},
				Commands: []*Command{
					{
						Name: "book",
						Arguments: []Argument{
							&StringArg{Name: "bookID"},
						},
						Commands: []*Command{
							{
								Name:   "delete",
								Action: action,
							},
						},
					},
				},
			},
		},
	}
}

func TestCommand_SubcommandAfterPositionalArgument(t *testing.T) {
	t.Run("nested resources", func(t *testing.T) {
		var authorID, bookID string
		var args []string

		cmd := nestedResourceCommand(func(_ context.Context, cmd *Command) error {
			authorID = cmd.StringArg("authorID")
			bookID = cmd.StringArg("bookID")
			args = cmd.Args().Slice()
			return nil
		})

		require.NoError(t, cmd.Run(buildTestContext(t),
			[]string{"manage", "author", "42", "book", "7", "delete"}))
		assert.Equal(t, "42", authorID)
		assert.Equal(t, "7", bookID)
		assert.Empty(t, args)
	})

	t.Run("remaining arguments reach the leaf command", func(t *testing.T) {
		var args []string

		cmd := nestedResourceCommand(func(_ context.Context, cmd *Command) error {
			args = cmd.Args().Slice()
			return nil
		})

		require.NoError(t, cmd.Run(buildTestContext(t),
			[]string{"manage", "author", "42", "book", "7", "delete", "extra"}))
		assert.Equal(t, []string{"extra"}, args)
	})

	t.Run("destination of an argument before a subcommand is set", func(t *testing.T) {
		var authorID string

		cmd := nestedResourceCommand(func(context.Context, *Command) error { return nil })
		cmd.Commands[0].Arguments = []Argument{
			&StringArg{Name: "authorID", Destination: &authorID},
		}

		require.NoError(t, cmd.Run(buildTestContext(t),
			[]string{"manage", "author", "42", "book", "7", "delete"}))
		assert.Equal(t, "42", authorID)
	})

	t.Run("flags are still parsed after the argument", func(t *testing.T) {
		var verbose bool
		var bookID string

		cmd := nestedResourceCommand(func(context.Context, *Command) error { return nil })
		cmd.Commands[0].Commands[0].Flags = []Flag{
			&BoolFlag{Name: "verbose", Destination: &verbose},
		}
		cmd.Commands[0].Commands[0].Arguments = []Argument{
			&StringArg{Name: "bookID", Destination: &bookID},
		}

		require.NoError(t, cmd.Run(buildTestContext(t),
			[]string{"manage", "author", "42", "book", "--verbose", "7", "delete"}))
		assert.True(t, verbose)
		assert.Equal(t, "7", bookID)
	})
}

func TestCommand_SubcommandAfterPositionalArgument_MultipleArguments(t *testing.T) {
	var first, second string

	cmd := &Command{
		Name: "app",
		Commands: []*Command{
			{
				Name: "pair",
				Arguments: []Argument{
					&StringArg{Name: "first"},
					&StringArg{Name: "second"},
				},
				Commands: []*Command{
					{
						Name: "show",
						Action: func(_ context.Context, cmd *Command) error {
							first = cmd.StringArg("first")
							second = cmd.StringArg("second")
							return nil
						},
					},
				},
			},
		},
	}

	require.NoError(t, cmd.Run(buildTestContext(t), []string{"app", "pair", "a", "b", "show"}))
	assert.Equal(t, "a", first)
	assert.Equal(t, "b", second)
}

func TestCommand_SubcommandAfterPositionalArgument_MultiValueArgument(t *testing.T) {
	var ids []string

	cmd := &Command{
		Name: "app",
		Commands: []*Command{
			{
				Name: "group",
				Arguments: []Argument{
					&StringArgs{Name: "ids", Max: -1},
				},
				Commands: []*Command{
					{
						Name: "list",
						Action: func(_ context.Context, cmd *Command) error {
							ids = cmd.StringArgs("ids")
							return nil
						},
					},
				},
			},
		},
	}

	// A glob argument stops at the subcommand rather than swallowing it.
	require.NoError(t, cmd.Run(buildTestContext(t), []string{"app", "group", "a", "b", "list"}))
	assert.Equal(t, []string{"a", "b"}, ids)
}

func TestCommand_SubcommandAfterPositionalArgument_SubcommandWithoutArgument(t *testing.T) {
	var ran bool

	cmd := &Command{
		Name: "app",
		Commands: []*Command{
			{
				Name: "author",
				Arguments: []Argument{
					&StringArg{Name: "authorID"},
				},
				Commands: []*Command{
					{
						Name:   "list",
						Action: func(context.Context, *Command) error { ran = true; return nil },
					},
				},
			},
		},
	}

	// Without the argument the subcommand is the first positional argument,
	// as before.
	require.NoError(t, cmd.Run(buildTestContext(t), []string{"app", "author", "list"}))
	assert.True(t, ran)
}

func TestCommand_SubcommandAfterPositionalArgument_RequiredArgumentMissing(t *testing.T) {
	var ran bool

	cmd := &Command{
		Name: "app",
		Commands: []*Command{
			{
				Name: "author",
				Arguments: []Argument{
					&StringArg{Name: "authorID", Required: true},
				},
				Commands: []*Command{
					{
						Name:   "delete",
						Action: func(context.Context, *Command) error { ran = true; return nil },
					},
				},
			},
		},
	}

	// "delete" names a subcommand, so it is not taken as the required
	// argument, and the argument is reported missing.
	err := cmd.Run(buildTestContext(t), []string{"app", "author", "delete"})
	assert.False(t, ran)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authorID")
}

func TestCommand_SubcommandAfterPositionalArgument_InvalidArgument(t *testing.T) {
	var ran bool

	cmd := &Command{
		Name: "app",
		Commands: []*Command{
			{
				Name: "author",
				Arguments: []Argument{
					&IntArg{Name: "authorID"},
				},
				Commands: []*Command{
					{
						Name:   "delete",
						Action: func(context.Context, *Command) error { ran = true; return nil },
					},
				},
			},
		},
	}

	err := cmd.Run(buildTestContext(t), []string{"app", "author", "notanint", "delete"})
	assert.False(t, ran)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "notanint")
}

func TestCommand_SubcommandAfterPositionalArgument_ActionWithoutSubcommand(t *testing.T) {
	var authorID string
	var args []string

	cmd := &Command{
		Name: "app",
		Commands: []*Command{
			{
				Name: "author",
				Arguments: []Argument{
					&StringArg{Name: "authorID"},
				},
				Commands: []*Command{
					{
						Name:   "delete",
						Action: func(context.Context, *Command) error { return nil },
					},
				},
				Action: func(_ context.Context, cmd *Command) error {
					authorID = cmd.StringArg("authorID")
					args = cmd.Args().Slice()
					return nil
				},
			},
		},
	}

	// With no subcommand the argument is still parsed by the action's
	// command, as before.
	require.NoError(t, cmd.Run(buildTestContext(t), []string{"app", "author", "42"}))
	assert.Equal(t, "42", authorID)
	assert.Empty(t, args)
}

func TestCommand_SubcommandAfterPositionalArgument_UnknownArgumentImplementation(t *testing.T) {
	var ran bool

	cmd := &Command{
		Name: "app",
		Commands: []*Command{
			{
				Name: "author",
				Arguments: []Argument{
					&opaqueArgument{},
				},
				Commands: []*Command{
					{
						Name:   "delete",
						Action: func(context.Context, *Command) error { ran = true; return nil },
					},
				},
			},
		},
	}

	// An Argument that does not report how much it consumes keeps the old
	// behaviour: the first positional argument names the subcommand.
	require.NoError(t, cmd.Run(buildTestContext(t), []string{"app", "author", "delete"}))
	assert.True(t, ran)
}

func TestCommand_SubcommandAfterPositionalArgument_Completion(t *testing.T) {
	var ran bool
	out := &bytes.Buffer{}

	cmd := nestedResourceCommand(func(context.Context, *Command) error {
		ran = true
		return nil
	})
	cmd.EnableShellCompletion = true
	cmd.Writer = out
	cmd.ErrWriter = out

	// Completing the leaf's subcommands reaches the leaf even though the
	// arguments come before it, and never runs the action.
	require.NoError(t, cmd.Run(buildTestContext(t),
		[]string{"manage", "author", "42", "book", "7", "delete", completionFlag}))
	assert.False(t, ran, "completion must not run the action")
	assert.NotContains(t, out.String(), "42", "an argument value must not become a suggestion")
	assert.NotContains(t, out.String(), "7", "an argument value must not become a suggestion")
}

// opaqueArgument is an Argument that does not implement argCounter.
type opaqueArgument struct{}

func (a *opaqueArgument) HasName(string) bool                { return false }
func (a *opaqueArgument) Parse(s []string) ([]string, error) { return s, nil }
func (a *opaqueArgument) Usage() string                      { return "" }
func (a *opaqueArgument) Get() any                           { return nil }
