// Command knit is a lightweight drop-in replacement for google/wire's code
// generator (knit gen) and a Test Impact Analysis tool that runs only the
// integration tests affected by a git diff (knit tia).
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
	"github.com/withnic/knit/internal/shared/graphschema"
	"github.com/withnic/knit/internal/tia"
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

const usage = `knit - wire-compatible DI code generator and integration test impact analysis

Usage:
  knit gen [flags] [packages...]   generate wire_gen.go and .knit/di-graph.json
  knit tia [flags]                 select (and run) integration tests affected by a git diff
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
	case "tia":
		return runTIA(ctx, args[1:], stdout, stderr)
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
	graphOut := fs.String("graph-out", graphschema.DefaultPath, "output path of the dependency graph JSON used by tia (empty to skip)")
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

func runTIA(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("tia", "tia [flags]", stderr)
	base := fs.String("base", "main", "base branch to compare against (changes since the merge base)")
	tag := fs.String("tag", tia.DefaultIntegrationTag, "build tag of integration test files")
	pattern := fs.String("pattern", tia.DefaultIntegrationSuffix, "file name suffix of integration test files")
	dirty := fs.Bool("dirty", true, "include uncommitted and untracked changes")
	execTests := fs.Bool("exec", false, "run the selected tests instead of printing the command")
	parallel := fs.Int("parallel-pkg", 1, "packages tested in parallel (go test -p)")
	hubs := fs.String("hub-exclude", "", "comma-separated hub function name patterns where traversal stops (e.g. NewServer,SetupRouter,Initialize*)")
	graphPath := fs.String("graph", graphschema.DefaultPath, "dependency graph JSON written by knit gen")
	verbose := fs.Bool("v", false, "explain why each test was selected")
	if code, ok := parseFlags(fs, args); !ok {
		return code
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "knit tia: unexpected arguments %v\n", fs.Args())
		fs.Usage()
		return exitUsage
	}

	root, err := tia.FindModuleRoot(".")
	if err != nil {
		fmt.Fprintf(stderr, "knit tia: %v\n", err)
		return exitError
	}
	res, err := tia.Plan(ctx, tia.PlanOptions{
		Root:        root,
		Base:        *base,
		Dirty:       *dirty,
		Tag:         *tag,
		Pattern:     *pattern,
		HubPatterns: splitList(*hubs),
		GraphPath:   *graphPath,
		Parallel:    *parallel,
	})
	if err != nil {
		fmt.Fprintf(stderr, "knit tia: %v\n", err)
		return exitError
	}
	for _, w := range res.Warnings {
		fmt.Fprintf(stderr, "knit tia: warning: %s\n", w)
	}
	if *verbose {
		explain(stderr, res)
	}
	if res.Command == nil {
		fmt.Fprintln(stderr, "knit tia: no affected integration tests")
		return exitOK
	}
	if !*execTests {
		fmt.Fprintln(stdout, tia.FormatCommand(res.Command))
		return exitOK
	}

	fmt.Fprintf(stderr, "knit tia: %s\n", tia.FormatCommand(res.Command))
	if err := tia.Exec(ctx, root, res.Command, stdout, stderr); err != nil {
		var ee *tia.ExitError
		if errors.As(err, &ee) {
			return ee.Code
		}
		fmt.Fprintf(stderr, "knit tia: %v\n", err)
		return exitError
	}
	return exitOK
}

// explain prints the selection rationale.
func explain(w io.Writer, res *tia.PlanResult) {
	sel := res.Selection
	fmt.Fprintf(w, "knit tia: %d changed ranges, %d seeds, %d affected symbols\n",
		len(res.Ranges), len(res.Seeds.Seeds), len(sel.Affected))
	for i := range res.Seeds.Seeds {
		s := &res.Seeds.Seeds[i]
		fmt.Fprintf(w, "  seed %s (%s: %s)\n", s.Symbol.ID, s.Reason, s.Cause)
	}
	for _, h := range sel.StoppedAtHubs {
		fmt.Fprintf(w, "  stopped at hub %s\n", h)
	}
	for _, inj := range sel.AffectedInjectors {
		fmt.Fprintf(w, "  affected injector %s\n", inj)
	}
	if sel.Fallback != "" {
		fmt.Fprintf(w, "  fallback: %s changed, selecting every integration test\n", sel.Fallback)
	}
	for i := range sel.Tests {
		t := &sel.Tests[i]
		fmt.Fprintf(w, "  test %s (%s: %s)\n", t.Symbol.ID, t.Reason, strings.Join(t.Path, " <- "))
	}
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
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
