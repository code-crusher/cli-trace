package cli

import (
	"maps"
	"slices"
	"strconv"
	"strings"
)

// Localizer renders a message from a catalogue. Keys are stable and listed by
// [Keys]; args fill the message's placeholders.
type Localizer interface {
	Message(key string, args MessageArgs) string
}

// MessageArgs are the values a message can refer to. Each field is available
// as a placeholder of the same name in lower case, e.g. {flag} or {count}.
type MessageArgs struct {
	Command  string // name of the command the message is about
	Flag     string // a flag name, as it is written on the command line
	Arg      string // an argument name
	Value    string // the offending input
	Choices  string // a list of related values, such as the other flags
	Source   string // where a value came from, such as an environment variable
	Type     string // the Go type a value is parsed as
	Cause    string // the underlying error, if any
	Count    int    // quantity used to pick a plural form
	Expected int    // expected quantity, where a message compares two
}

// Form is a CLDR plural category.
type Form uint8

const (
	Other Form = iota
	Zero
	One
	Two
	Few
	Many
)

// Text is one message in all the plural forms a language needs. Other is
// required; the rest fall back to it.
type Text struct {
	Zero, One, Two, Few, Many, Other string
}

func (t Text) form(f Form) string {
	var s string
	switch f {
	case Zero:
		s = t.Zero
	case One:
		s = t.One
	case Two:
		s = t.Two
	case Few:
		s = t.Few
	case Many:
		s = t.Many
	}
	if s == "" {
		return t.Other
	}
	return s
}

// Catalog is a map-based [Localizer] for one language. Messages it does not
// have fall back to [English], so a partial translation is usable.
type Catalog struct {
	Tag      string           // BCP 47 tag, e.g. "de" or "pt-BR"
	Plural   func(n int) Form // nil means every count uses Other
	Messages map[string]Text
	Fallback Localizer // used for missing keys; nil means English
}

// Message renders key, falling back when the catalogue lacks it.
func (c *Catalog) Message(key string, a MessageArgs) string {
	t, ok := c.Messages[key]
	if !ok {
		if c.Fallback != nil {
			return c.Fallback.Message(key, a)
		}
		if c != English {
			return English.Message(key, a)
		}
		return key
	}
	f := Other
	if c.Plural != nil {
		f = c.Plural(a.Count)
	}
	return expand(t.form(f), a)
}

// OneOther is the plural rule for English, German, Dutch, Swedish and other
// languages that only distinguish one from many.
func OneOther(n int) Form {
	if n == 1 {
		return One
	}
	return Other
}

func expand(s string, a MessageArgs) string {
	if !strings.Contains(s, "{") {
		return s
	}
	return strings.NewReplacer(
		"{command}", a.Command,
		"{flag}", a.Flag,
		"{arg}", a.Arg,
		"{value}", a.Value,
		"{choices}", a.Choices,
		"{source}", a.Source,
		"{type}", a.Type,
		"{cause}", a.Cause,
		"{count}", strconv.Itoa(a.Count),
		"{expected}", strconv.Itoa(a.Expected),
	).Replace(s)
}

// English is the built-in catalogue and the final fallback for every other.
var English = &Catalog{
	Tag:    "en",
	Plural: OneOther,
	Messages: map[string]Text{
		"error.usage":             {Other: "Incorrect Usage: {cause}"},
		"error.unknown_flag":      {Other: "flag provided but not defined: -{flag}"},
		"error.missing_value":     {Other: "flag needs an argument: {flag}"},
		"error.invalid_value":     {Other: `invalid value "{value}" for flag -{flag}: {cause}`},
		"error.invalid_arg_value": {Other: `invalid value "{value}" for argument {arg}: {cause}`},
		"error.required_flag": {
			One:   `Required flag "{flag}" not set`,
			Other: `Required flags "{flag}" not set`,
		},
		"error.required_arg": {
			One:   `Required argument "{arg}" not set`,
			Other: `Required arguments "{arg}" not set`,
		},
		"error.flag_conflict":  {Other: "option {flag} cannot be set along with option {choices}"},
		"error.flags_both_set": {Other: "cannot set both flags `{flag}` and `{choices}`"},
		"error.flag_requires":  {Other: "one of these flags needs to be provided: {choices}"},
		"error.args_count": {
			Other: "sufficient count of arg {arg} not provided, given {count} expected {expected}",
		},
		"error.duplicate_flag": {Other: "can't duplicate this flag"},
		"error.source_value": {
			Other: `could not parse "{value}" as {type} value from {source} for flag {flag}: {cause}`,
		},
		"error.no_help_topic":            {Other: "No help topic for '{value}'"},
		"error.no_help_topic_suggestion": {Other: "No help topic for '{value}'. {choices}"},

		"warning.command_deprecated": {Other: `Command "{command}" is deprecated, {value}`},
		"warning.flag_deprecated":    {Other: "Flag {flag} has been deprecated, {value}"},
	},
}

// Keys returns every key the library uses, sorted. A translation is complete
// when it covers all of them.
func Keys() []string {
	return slices.Sorted(maps.Keys(English.Messages))
}

// DefaultLocalizer renders the messages the library prints for commands that
// do not set a [Command.Localizer]. It defaults to [English]; set it once at
// startup to translate the output of every command.
//
// It does not change the text returned by an error's Error method, which is
// always English so that callers can match on it. Use [Command.Localize] to
// render an error in a command's language.
var DefaultLocalizer Localizer = English

// localizer returns the [Localizer] of the nearest command in the lineage that
// sets one, or [DefaultLocalizer].
func (cmd *Command) localizer() Localizer {
	for c := cmd; c != nil; c = c.parent {
		if c.Localizer != nil {
			return c.Localizer
		}
	}
	if DefaultLocalizer != nil {
		return DefaultLocalizer
	}
	return English
}

// Message renders a catalogue message in the command's language. key is one of
// [Keys] and args fill the message's placeholders.
func (cmd *Command) Message(key string, args MessageArgs) string {
	return cmd.localizer().Message(key, args)
}

// Localize renders err in the command's language. Errors the library did not
// create are returned as they are.
func (cmd *Command) Localize(err error) string {
	return localizeError(err, cmd.localizer())
}

// localizeError renders err with loc when the library created it, and returns
// err's own message otherwise.
func localizeError(err error, loc Localizer) string {
	if err == nil {
		return ""
	}
	if loc == nil {
		loc = English
	}
	if le, ok := err.(localizable); ok {
		return le.localize(loc)
	}
	return err.Error()
}

// localizable is implemented by the errors the library itself returns, so that
// they can be rendered in any language. Their Error method stays English.
type localizable interface {
	localize(Localizer) string
}

// usageErrorText renders err as the "Incorrect Usage" line printed when the
// command line is wrong.
func (cmd *Command) usageErrorText(err error) string {
	return cmd.Message("error.usage", MessageArgs{Cause: cmd.Localize(err)})
}

// DetectLocales returns the user's preferred message languages as BCP 47 tags,
// most preferred first, following POSIX and GNU gettext rules: LC_ALL, then
// LC_MESSAGES, then LANG pick the locale, and LANGUAGE, a colon-separated
// list, takes priority unless the locale is C or POSIX. It returns nil when
// the user has asked for the default language.
//
// lookup is usually [os.LookupEnv]; tests pass their own.
func DetectLocales(lookup func(string) (string, bool)) []string {
	var locale string
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v, ok := lookup(name); ok && v != "" {
			locale = v
			break
		}
	}
	tag := normalizeLocale(locale)
	if tag == "" {
		return nil
	}

	var tags []string
	if list, ok := lookup("LANGUAGE"); ok {
		for _, l := range strings.Split(list, ":") {
			if t := normalizeLocale(l); t != "" {
				tags = append(tags, t)
			}
		}
	}
	return append(tags, tag)
}

// normalizeLocale turns a POSIX locale such as "de_DE.UTF-8@euro" into "de-DE".
// C and POSIX map to "".
func normalizeLocale(locale string) string {
	if i := strings.IndexAny(locale, ".@"); i >= 0 {
		locale = locale[:i]
	}
	if locale == "C" || locale == "POSIX" {
		return ""
	}
	return strings.ReplaceAll(locale, "_", "-")
}

// SelectLocalizer returns the catalogue that best matches the user's locale,
// or [English]. An exact tag match wins over a language-only match, so
// "pt-BR" prefers a pt-BR catalogue to a pt one.
//
// lookup is usually [os.LookupEnv]. Use it to pick a Localizer at startup:
//
//	cmd.Localizer = cli.SelectLocalizer(os.LookupEnv, de.Catalog)
func SelectLocalizer(lookup func(string) (string, bool), catalogs ...*Catalog) Localizer {
	for _, want := range DetectLocales(lookup) {
		if c := matchCatalog(want, catalogs); c != nil {
			return c
		}
	}
	return English
}

func matchCatalog(want string, catalogs []*Catalog) *Catalog {
	for _, c := range catalogs {
		if strings.EqualFold(c.Tag, want) {
			return c
		}
	}
	base, _, _ := strings.Cut(want, "-")
	for _, c := range catalogs {
		if strings.EqualFold(c.Tag, base) {
			return c
		}
	}
	return nil
}
