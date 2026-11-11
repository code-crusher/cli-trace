// Command example-i18n shows how to report errors in the user's language.
// Run it with LANG=de_DE.UTF-8 and an unknown flag to see a German message.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"
	"github.com/urfave/cli/v3/i18n/locale/de"
)

func main() {
	cmd := &cli.Command{
		Name:      "example-i18n",
		Usage:     "say hello",
		Localizer: cli.SelectLocalizer(os.LookupEnv, de.Catalog),
		Action: func(context.Context, *cli.Command) error {
			fmt.Println("Hallo")
			return nil
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, cmd.Localize(err))
		os.Exit(1)
	}
}
