//go:build !urfave_cli_no_completion

package cli_test

import (
	"context"

	// Alias the package import to make the examples runnable on pkg.go.dev.
	//
	// See issue #1811.
	cli "github.com/urfave/cli/v3"
)

func ExampleCommand_Run_shellComplete_bash_withShortFlag() {
	cmd := &cli.Command{
		Name:                  "greet",
		EnableShellCompletion: true,
		Flags: []cli.Flag{
			&cli.Int64Flag{
				Name:    "other",
				Aliases: []string{"o"},
			},
			&cli.StringFlag{
				Name:    "xyz",
				Aliases: []string{"x"},
			},
		},
	}

	_ = cmd.Run(context.Background(), []string{"greet", "-", "--generate-shell-completion"})
	// Output:
	// --other
	// --xyz
	// --help:show help
}

func ExampleCommand_Run_shellComplete_bash_withLongFlag() {
	cmd := &cli.Command{
		Name:                  "greet",
		EnableShellCompletion: true,
		Flags: []cli.Flag{
			&cli.Int64Flag{
				Name:    "other",
				Aliases: []string{"o"},
			},
			&cli.StringFlag{
				Name:    "xyz",
				Aliases: []string{"x"},
			},
			&cli.StringFlag{
				Name: "some-flag,s",
			},
			&cli.StringFlag{
				Name: "similar-flag",
			},
		},
	}

	_ = cmd.Run(context.Background(), []string{"greet", "--s", "--generate-shell-completion"})
	// Output:
	// --some-flag
	// --similar-flag
}

func ExampleCommand_Run_shellComplete_bash_withMultipleLongFlag() {
	cmd := &cli.Command{
		Name:                  "greet",
		EnableShellCompletion: true,
		Flags: []cli.Flag{
			&cli.Int64Flag{
				Name:    "int-flag",
				Aliases: []string{"i"},
			},
			&cli.StringFlag{
				Name:    "string",
				Aliases: []string{"s"},
			},
			&cli.StringFlag{
				Name: "string-flag-2",
			},
			&cli.StringFlag{
				Name: "similar-flag",
			},
			&cli.StringFlag{
				Name: "some-flag",
			},
		},
	}

	_ = cmd.Run(context.Background(), []string{"greet", "--st", "--generate-shell-completion"})
	// Output:
	// --string
	// --string-flag-2
}

func ExampleCommand_Run_shellComplete_bash() {
	cmd := &cli.Command{
		Name:                  "greet",
		EnableShellCompletion: true,
		Commands: []*cli.Command{
			{
				Name:        "describeit",
				Aliases:     []string{"d"},
				Usage:       "use it to see a description",
				Description: "This is how we describe describeit the function",
			}, {
				Name:        "next",
				Usage:       "next example",
				Description: "more stuff to see when generating shell completion",
			},
		},
	}

	_ = cmd.Run(context.Background(), []string{"greet", "--generate-shell-completion"})
	// Output:
	// describeit:use it to see a description
	// next:next example
	// help:Shows a list of commands or help for one command
}
