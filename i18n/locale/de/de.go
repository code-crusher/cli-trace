// Package de is the German catalogue. It is plain Go data, so importing it
// adds no JSON decoder to the binary.
package de

import "github.com/urfave/cli/v3"

// Catalog translates every message the library prints. Use it at startup:
//
//	cmd.Localizer = cli.SelectLocalizer(os.LookupEnv, de.Catalog)
var Catalog = &cli.Catalog{
	Tag:    "de",
	Plural: cli.OneOther,
	Messages: map[string]cli.Text{
		"error.usage":             {Other: "Falsche Verwendung: {cause}"},
		"error.unknown_flag":      {Other: "Flagge angegeben, aber nicht definiert: -{flag}"},
		"error.missing_value":     {Other: "Flagge benötigt ein Argument: {flag}"},
		"error.invalid_value":     {Other: `ungültiger Wert "{value}" für Flagge -{flag}: {cause}`},
		"error.invalid_arg_value": {Other: `ungültiger Wert "{value}" für Argument {arg}: {cause}`},
		"error.required_flag": {
			One:   `Erforderliche Flagge "{flag}" nicht gesetzt`,
			Other: `Erforderliche Flaggen "{flag}" nicht gesetzt`,
		},
		"error.required_arg": {
			One:   `Erforderliches Argument "{arg}" nicht gesetzt`,
			Other: `Erforderliche Argumente "{arg}" nicht gesetzt`,
		},
		"error.flag_conflict":  {Other: "Option {flag} kann nicht zusammen mit Option {choices} gesetzt werden"},
		"error.flags_both_set": {Other: "die Flaggen `{flag}` und `{choices}` können nicht beide gesetzt werden"},
		"error.flag_requires":  {Other: "eine dieser Flaggen muss angegeben werden: {choices}"},
		"error.args_count": {
			Other: "zu wenige Werte für Argument {arg}: {count} angegeben, {expected} erwartet",
		},
		"error.duplicate_flag": {Other: "diese Flagge kann nicht wiederholt werden"},
		"error.source_value": {
			Other: `"{value}" konnte nicht als {type}-Wert aus {source} für Flagge {flag} gelesen werden: {cause}`,
		},
		"error.no_help_topic":            {Other: "Kein Hilfethema für '{value}'"},
		"error.no_help_topic_suggestion": {Other: "Kein Hilfethema für '{value}'. {choices}"},

		"warning.command_deprecated": {Other: `Befehl "{command}" ist veraltet, {value}`},
		"warning.flag_deprecated":    {Other: "Flagge {flag} ist veraltet, {value}"},
	},
}
