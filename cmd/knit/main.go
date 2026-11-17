// Command knit is a lightweight drop-in replacement for google/wire's code
// generator. knit gen writes wire_gen.go for every injector and the
// dependency graph .knit/di-graph.json.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/withnic/knit/internal/di"
	"github.com/withnic/knit/internal/graphschema"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

// Exit codes.
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

const usage = `knit - wire-compatible DI code generator

Usage:
  knit gen [flags] [packages...]   generate wire_gen.go and .knit/di-graph.json
  knit version                     print the version

Run "knit <command> -h" for the flags of a command.
`

// run executes the CLI and returns the process exit code.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
	switch args[0] {
	case "gen":
		return runGen(ctx, args[1:], stdout, stderr)
	case "version", "-version", "--version":
		fmt.Fprintln(stdout, "knit", version)
		return exitOK
	case "help", "-h", "-help", "--help":
		fmt.Fprint(stdout, usage)
		return exitOK
	default:
		fmt.Fprintf(stderr, "knit: unknown command %q\n\n%s", args[0], usage)
		return exitUsage
	}
}

// newFlagSet returns a FlagSet that reports errors instead of exiting.
func newFlagSet(name, synopsis string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: knit %s\n\nFlags:\n", synopsis)
		fs.PrintDefaults()
	}
	return fs
}

// parseFlags parses args and maps -h and invalid flags to exit codes.
func parseFlags(fs *flag.FlagSet, args []string) (code int, ok bool) {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK, false
		}
		return exitUsage, false
	}
	return exitOK, true
}

func runGen(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("gen", "gen [flags] [packages...]", stderr)
	out := fs.String("out", di.DefaultOutFile, "name of the generated file in each package directory")
	graphOut := fs.String("graph-out", graphschema.DefaultPath, "output path of the dependency graph JSON (empty to skip)")
	dryRun := fs.Bool("dry-run", false, "print the generated code to stdout instead of writing files")
	if code, ok := parseFlags(fs, args); !ok {
		return code
	}

	res, err := di.Gen(ctx, di.GenOptions{Patterns: fs.Args(), OutFile: *out})
	if err != nil {
		fmt.Fprintf(stderr, "knit gen: %v\n", err)
		return exitError
	}

	if *dryRun {
		for i, f := range res.Files {
			if i > 0 {
				fmt.Fprintln(stdout)
			}
			fmt.Fprintf(stdout, "// ==> %s <==\n", displayPath(f.Path))
			if _, err := stdout.Write(f.Content); err != nil {
				fmt.Fprintf(stderr, "knit gen: %v\n", err)
				return exitError
			}
		}
		return exitOK
	}

	if err := res.Write(*graphOut); err != nil {
		fmt.Fprintf(stderr, "knit gen: %v\n", err)
		return exitError
	}
	for _, f := range res.Files {
		fmt.Fprintf(stderr, "knit gen: wrote %s\n", displayPath(f.Path))
	}
	if *graphOut != "" {
		fmt.Fprintf(stderr, "knit gen: wrote %s\n", *graphOut)
	}
	return exitOK
}

// displayPath shortens p relative to the working directory when possible.
func displayPath(p string) string {
	wd, err := os.Getwd()
	if err != nil {
		return p
	}
	if rel, err := filepath.Rel(wd, p); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return p
}
