package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// OsExiter is the function used when the app exits. If not set defaults to os.Exit.
var OsExiter = os.Exit

// ErrWriter is used to write errors to the user. This can be anything
// implementing the io.Writer interface and defaults to os.Stderr.
var ErrWriter io.Writer = os.Stderr

// MultiError is an error that wraps multiple errors.
type MultiError interface {
	error
	Errors() []error
}

// newMultiError creates a new MultiError. Pass in one or more errors.
func newMultiError(err ...error) MultiError {
	ret := multiError(err)
	return &ret
}

type multiError []error

// Error implements the error interface.
func (m *multiError) Error() string {
	errs := make([]string, len(*m))
	for i, err := range *m {
		errs[i] = err.Error()
	}

	return strings.Join(errs, "\n")
}

// Errors returns a copy of the errors slice
func (m *multiError) Errors() []error {
	errs := make([]error, len(*m))
	copy(errs, *m)
	return errs
}

type requiredFlagsErr interface {
	error
}

type errRequiredFlags struct {
	missingFlags []string
}

func (e *errRequiredFlags) Error() string {
	return e.localize(English)
}

func (e *errRequiredFlags) localize(loc Localizer) string {
	return loc.Message("error.required_flag", MessageArgs{
		Flag:  strings.Join(e.missingFlags, ", "),
		Count: len(e.missingFlags),
	})
}

type requiredArgumentsErr interface {
	error
}

type errRequiredArguments struct {
	missingArguments []string
}

func (e *errRequiredArguments) Error() string {
	return e.localize(English)
}

func (e *errRequiredArguments) localize(loc Localizer) string {
	return loc.Message("error.required_arg", MessageArgs{
		Arg:   strings.Join(e.missingArguments, ", "),
		Count: len(e.missingArguments),
	})
}

type mutuallyExclusiveGroup struct {
	flag1Name string
	flag2Name string
}

func (e *mutuallyExclusiveGroup) Error() string {
	return e.localize(English)
}

func (e *mutuallyExclusiveGroup) localize(loc Localizer) string {
	return loc.Message("error.flag_conflict", MessageArgs{
		Flag:    e.flag1Name,
		Choices: e.flag2Name,
	})
}

type mutuallyExclusiveGroupRequiredFlag struct {
	flags *MutuallyExclusiveFlags
}

func (e *mutuallyExclusiveGroupRequiredFlag) Error() string {
	return e.localize(English)
}

func (e *mutuallyExclusiveGroupRequiredFlag) localize(loc Localizer) string {
	var missingFlags []string
	for _, grpf := range e.flags.Flags {
		var grpString []string
		for _, f := range grpf {
			grpString = append(grpString, f.Names()...)
		}
		missingFlags = append(missingFlags, strings.Join(grpString, " "))
	}

	return loc.Message("error.flag_requires", MessageArgs{
		Choices: strings.Join(missingFlags, ", "),
	})
}

// errUnknownFlag reports a flag the command does not define. name is written
// without its dashes, as the parser receives it.
type errUnknownFlag struct {
	name string
}

func newErrUnknownFlag(name string) *errUnknownFlag {
	return &errUnknownFlag{name: name}
}

func (e *errUnknownFlag) Error() string {
	return e.localize(English)
}

func (e *errUnknownFlag) localize(loc Localizer) string {
	return loc.Message("error.unknown_flag", MessageArgs{Flag: e.name})
}

// errMissingValue reports a flag given without the value it needs. flag is
// written with its dashes, as it appeared on the command line.
type errMissingValue struct {
	flag string
}

func newErrMissingValue(flag string) *errMissingValue {
	return &errMissingValue{flag: flag}
}

func (e *errMissingValue) Error() string {
	return e.localize(English)
}

func (e *errMissingValue) localize(loc Localizer) string {
	return loc.Message("error.missing_value", MessageArgs{Flag: e.flag})
}

// errInvalidArgValue reports an argument whose value could not be parsed.
type errInvalidArgValue struct {
	value string
	arg   string
	err   error
}

func (e *errInvalidArgValue) Error() string {
	return e.localize(English)
}

func (e *errInvalidArgValue) Unwrap() error {
	return e.err
}

func (e *errInvalidArgValue) localize(loc Localizer) string {
	return loc.Message("error.invalid_arg_value", MessageArgs{
		Value: e.value,
		Arg:   e.arg,
		Cause: e.err.Error(),
	})
}

// errInvalidFlagValue reports a flag whose value could not be parsed. flag is
// written without its dashes.
type errInvalidFlagValue struct {
	value string
	flag  string
	err   error
}

func (e *errInvalidFlagValue) Error() string {
	return e.localize(English)
}

func (e *errInvalidFlagValue) Unwrap() error {
	return e.err
}

func (e *errInvalidFlagValue) localize(loc Localizer) string {
	return loc.Message("error.invalid_value", MessageArgs{
		Value: e.value,
		Flag:  e.flag,
		Cause: e.err.Error(),
	})
}

// errDuplicateFlag reports a flag that may be given only once, given twice.
type errDuplicateFlag struct{}

func (e *errDuplicateFlag) Error() string {
	return e.localize(English)
}

func (e *errDuplicateFlag) localize(loc Localizer) string {
	return loc.Message("error.duplicate_flag", MessageArgs{})
}

// errSourceValue reports a value from a flag's Sources, such as an
// environment variable, that could not be parsed.
type errSourceValue struct {
	value  string
	typ    string
	source string
	flag   string
	err    error
}

func (e *errSourceValue) Error() string {
	return e.localize(English)
}

func (e *errSourceValue) Unwrap() error {
	return e.err
}

func (e *errSourceValue) localize(loc Localizer) string {
	return loc.Message("error.source_value", MessageArgs{
		Value:  e.value,
		Type:   e.typ,
		Source: e.source,
		Flag:   e.flag,
		Cause:  e.err.Error(),
	})
}

// errBothSetFlags reports a flag and its inverse set in the same command.
// Both names are written with their dashes.
type errBothSetFlags struct {
	flag    string
	inverse string
}

func (e *errBothSetFlags) Error() string {
	return e.localize(English)
}

func (e *errBothSetFlags) localize(loc Localizer) string {
	return loc.Message("error.flags_both_set", MessageArgs{
		Flag:    e.flag,
		Choices: e.inverse,
	})
}

// errArgsCount reports too few values for an argument that has a minimum.
type errArgsCount struct {
	arg      string
	count    int
	expected int
}

func (e *errArgsCount) Error() string {
	return e.localize(English)
}

func (e *errArgsCount) localize(loc Localizer) string {
	return loc.Message("error.args_count", MessageArgs{
		Arg:      e.arg,
		Count:    e.count,
		Expected: e.expected,
	})
}

// errNoHelpTopic reports a help request for a command that does not exist.
// suggestion, when set, is the already-rendered "Did you mean" hint.
type errNoHelpTopic struct {
	topic      string
	suggestion string
}

func (e *errNoHelpTopic) Error() string {
	return e.localize(English)
}

func (e *errNoHelpTopic) localize(loc Localizer) string {
	if e.suggestion != "" {
		return loc.Message("error.no_help_topic_suggestion", MessageArgs{
			Value:   e.topic,
			Choices: e.suggestion,
		})
	}
	return loc.Message("error.no_help_topic", MessageArgs{Value: e.topic})
}

// ErrorFormatter is the interface that will suitably format the error output
type ErrorFormatter interface {
	Format(s fmt.State, verb rune)
}

// ExitCoder is the interface checked by `Command` for a custom exit code.
type ExitCoder interface {
	error
	ExitCode() int
}

type exitError struct {
	exitCode int
	err      error
}

// Exit wraps a message and exit code into an error, which by default is
// handled with a call to os.Exit during default error handling.
//
// This is the simplest way to trigger a non-zero exit code for a Command without
// having to call os.Exit manually. During testing, this behavior can be avoided
// by overriding the ExitErrHandler function on a Command or the package-global
// OsExiter function.
func Exit(message any, exitCode int) ExitCoder {
	var err error

	switch e := message.(type) {
	case ErrorFormatter:
		err = fmt.Errorf("%+v", message)
	case error:
		err = e
	default:
		err = fmt.Errorf("%+v", message)
	}

	return &exitError{
		err:      err,
		exitCode: exitCode,
	}
}

func (ee *exitError) Error() string {
	return ee.err.Error()
}

func (ee *exitError) ExitCode() int {
	return ee.exitCode
}

// Unwrap returns the error given to [Exit], so that [errors.Is] and
// [errors.As] can inspect it through the ExitCoder wrapper.
func (ee *exitError) Unwrap() error {
	return ee.err
}

func (ee *exitError) localize(loc Localizer) string {
	return localizeError(ee.err, loc)
}

// HandleExitCoder handles errors implementing ExitCoder by printing their
// message and calling OsExiter with the given exit code.
//
// If the given error instead implements MultiError, each error will be checked
// for the ExitCoder interface, and OsExiter will be called with the last exit
// code found, or exit code 1 if no ExitCoder is found.
//
// An ExitCoder that is wrapped with fmt.Errorf("%w") is handled the same way,
// so adding context to an error does not lose its exit code.
//
// This function is the default error-handling behavior for a Command.
func HandleExitCoder(err error) {
	handleExitCoder(err, DefaultLocalizer)
}

// handleExitCoder is [HandleExitCoder] rendering messages with loc. A Command
// passes its own Localizer, so that errors it prints are in its language.
func handleExitCoder(err error, loc Localizer) {
	if err == nil {
		return
	}

	_, isExitCoder := err.(ExitCoder)
	// A MultiError is handled as a whole: unwrapping it would find its first
	// ExitCoder and lose the documented "last exit code wins".
	if multiErr, ok := err.(MultiError); ok && !isExitCoder {
		code := handleMultiError(multiErr, loc)
		OsExiter(code)
		return
	}

	if exitErr, ok := errors.AsType[ExitCoder](err); ok {
		if msg := err.Error(); msg != "" {
			if _, ok := exitErr.(ErrorFormatter); ok {
				_, _ = fmt.Fprintf(ErrWriter, "%+v\n", err)
			} else {
				_, _ = fmt.Fprintln(ErrWriter, localizeError(err, loc))
			}
		}
		OsExiter(exitErr.ExitCode())
		return
	}
}

func handleMultiError(multiErr MultiError, loc Localizer) int {
	code := 1
	for _, merr := range multiErr.Errors() {
		if multiErr2, ok := merr.(MultiError); ok {
			code = handleMultiError(multiErr2, loc)
		} else if merr != nil {
			if _, ok := merr.(ErrorFormatter); ok {
				_, _ = fmt.Fprintf(ErrWriter, "%+v\n", merr)
			} else {
				_, _ = fmt.Fprintln(ErrWriter, localizeError(merr, loc))
			}
			if exitErr, ok := errors.AsType[ExitCoder](merr); ok {
				code = exitErr.ExitCode()
			}
		}
	}
	return code
}
