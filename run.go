package main

import (
	"context"
	"fmt"
	"io"

	command "github.com/gloo-foo/cmd-basename"
	gloo "github.com/gloo-foo/framework"
	"github.com/spf13/afero"
	"github.com/urfave/cli/v3"
)

const (
	flagMultiple = "multiple"
	flagSuffix   = "suffix"
	flagZero     = "zero"
)

// usageText is the command's multi-line usage synopsis, shown in --help.
// cli/v3 indents the whole block by 3 spaces, so these lines are flush-left to
// stay aligned in the rendered output.
const usageText = `basename NAME [SUFFIX]
basename OPTION... NAME...

Print NAME with any leading directory components removed.
If specified, also remove a trailing SUFFIX.`

// Error is the sole error type the wrapper emits, so every failure path is
// testable with errors.Is.
type Error string

func (e Error) Error() string { return string(e) }

const (
	// ErrMissingOperand is returned when no NAME operand is given.
	ErrMissingOperand Error = "missing operand"
	// ErrExtraOperand is returned when a third bare operand follows NAME and
	// SUFFIX without -a/-s (GNU basename rejects it).
	ErrExtraOperand Error = "extra operand"
)

// init replaces urfave/cli's default --version/-v flag with a --version-only
// flag, freeing the single-letter -v for command flags while still exposing
// the injected build version.
func init() {
	cli.VersionFlag = &cli.BoolFlag{Name: "version", Usage: "print version information and exit"}
}

// run builds and executes the basename CLI against the injected version, I/O,
// and filesystem, returning the process exit code. basename operates on its
// NAME operands, not on stdin or the filesystem; both are injected only to keep
// a uniform, testable wiring shape across the command wrappers.
func run(version string, args []string, _ io.Reader, stdout, stderr io.Writer, _ afero.Fs) int {
	cmd := newCommand(version, stdout)
	cmd.Writer = stdout
	cmd.ErrWriter = stderr
	if err := cmd.Run(context.Background(), args); err != nil {
		_, _ = fmt.Fprintf(stderr, "basename: %v\n", err)
		return 1
	}
	return 0
}

func newCommand(version string, stdout io.Writer) *cli.Command {
	return &cli.Command{
		Name:            "basename",
		Version:         version,
		Usage:           "strip directory and suffix from filenames",
		UsageText:       usageText,
		HideHelpCommand: true,
		// Allow GNU-style bundling of single-letter flags (e.g. -az == -a -z).
		UseShortOptionHandling: true,
		// Keep exit handling in run() rather than letting urfave/cli call
		// os.Exit, so the exit code stays testable.
		ExitErrHandler: func(context.Context, *cli.Command, error) {},
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: flagMultiple, Aliases: []string{"a"}, Usage: "support multiple NAME arguments, treating each as a NAME"},
			&cli.StringFlag{Name: flagSuffix, Aliases: []string{"s"}, Usage: "remove a trailing SUFFIX; implies -a"},
			&cli.BoolFlag{Name: flagZero, Aliases: []string{"z"}, Usage: "end each output line with NUL, not newline"},
		},
		Action: action(stdout),
	}
}

func action(stdout io.Writer) cli.ActionFunc {
	return func(_ context.Context, cmd *cli.Command) error {
		names, suffix, err := operands(cmd)
		if err != nil {
			return err
		}
		results, err := strip(names, suffix)
		if err != nil {
			return err
		}
		return emit(stdout, results, terminator(cmd))
	}
}

// operands resolves the NAME list and SUFFIX from the parsed flags and
// positional arguments, applying GNU basename's two operand grammars:
//
//   - -a/--multiple (or -s): every positional is a NAME; SUFFIX comes from -s.
//   - otherwise:             NAME [SUFFIX], i.e. exactly one or two positionals.
func operands(cmd *cli.Command) ([]string, string, error) {
	if cmd.Bool(flagMultiple) || cmd.IsSet(flagSuffix) {
		return multipleOperands(cmd)
	}
	return singleOperands(cmd)
}

func multipleOperands(cmd *cli.Command) ([]string, string, error) {
	if cmd.NArg() == 0 {
		return nil, "", ErrMissingOperand
	}
	return cmd.Args().Slice(), cmd.String(flagSuffix), nil
}

func singleOperands(cmd *cli.Command) ([]string, string, error) {
	switch cmd.NArg() {
	case 1:
		return []string{cmd.Args().Get(0)}, "", nil
	case 2:
		return []string{cmd.Args().Get(0)}, cmd.Args().Get(1), nil
	case 0:
		return nil, "", ErrMissingOperand
	default:
		return nil, "", ErrExtraOperand
	}
}

// collect drains a gloo pipeline to a []byte-per-line slice. It is a package
// var so a test can substitute a failing pipeline and exercise strip's error
// path; the production value is the real framework Collect.
var collect = func(names []string, suffix string) (any, error) {
	return gloo.Chain(gloo.SliceSource(toLines(names))).
		To(command.Basename(suffixOptions(suffix)...)).
		Collect()
}

// strip runs each NAME through the cmd-basename command, which removes the
// directory component and (when set) the trailing suffix, returning one output
// line per NAME, in order.
func strip(names []string, suffix string) ([][]byte, error) {
	out, err := collect(names, suffix)
	if err != nil {
		return nil, err
	}
	return out.([][]byte), nil
}

func suffixOptions(suffix string) []any {
	if suffix == "" {
		return nil
	}
	return []any{command.BasenameSuffix(suffix)}
}

func toLines(names []string) [][]byte {
	lines := make([][]byte, len(names))
	for i, name := range names {
		lines[i] = []byte(name)
	}
	return lines
}

// terminator selects the byte that ends each output record: NUL under -z,
// newline otherwise (matching GNU basename's --zero behaviour).
func terminator(cmd *cli.Command) byte {
	if cmd.Bool(flagZero) {
		return 0
	}
	return '\n'
}

func emit(stdout io.Writer, results [][]byte, sep byte) error {
	for _, result := range results {
		if _, err := stdout.Write(append(result, sep)); err != nil {
			return err
		}
	}
	return nil
}
