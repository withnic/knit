package di

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/withnic/knit/internal/graphschema"
)

// GenOptions configures Gen.
type GenOptions struct {
	// Mode selects the injector dialect. The zero Mode means KnitMode.
	Mode Mode
	// Dir is the directory package patterns are resolved from. Empty means
	// the current directory.
	Dir string
	// Patterns are the package patterns to generate code for. Empty means
	// "./...".
	Patterns []string
	// OutFile is the base name of the generated file in each package
	// directory. Empty means Mode.OutFile.
	OutFile string
	// Env replaces the environment of the underlying go command when non-nil.
	Env []string
	// Now returns the graph timestamp. Nil means time.Now.
	Now func() time.Time
}

// GeneratedFile is one generated source file.
type GeneratedFile struct {
	// Path is the absolute path the file should be written to.
	Path string
	// Content is the formatted Go source.
	Content []byte
}

// GenResult is the output of Gen.
type GenResult struct {
	// Files lists one generated file per package, sorted by Path.
	Files []GeneratedFile
	// Graph is the di-graph.json document for every injector.
	Graph *graphschema.Graph
}

// Gen loads the packages matching opts.Patterns, resolves every injector and
// renders one file per package together with the dependency graph. Nothing is
// written to disk; see GenResult.Write.
//
// Errors from all injectors are reported together, sorted, so a single run
// surfaces every problem.
func Gen(ctx context.Context, opts GenOptions) (*GenResult, error) {
	patterns := opts.Patterns
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	mode := opts.Mode.orDefault()
	outFile := opts.OutFile
	if outFile == "" {
		outFile = mode.OutFile
	}
	if filepath.Base(outFile) != outFile || filepath.Ext(outFile) != ".go" {
		return nil, fmt.Errorf("output file %q must be a plain .go file name", outFile)
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}

	l := NewLoader(mode, opts.Dir, opts.Env)
	pkgs, err := l.Load(ctx, patterns...)
	if err != nil {
		return nil, err
	}

	var (
		resolved   []*ResolvedInjector
		errs       []error
		modulePath string
	)
	for _, p := range pkgs {
		if modulePath == "" && p.Module != nil {
			modulePath = p.Module.Path
		}
		injs, err := l.Injectors(ctx, p)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, inj := range injs {
			r, err := Resolve(inj)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			resolved = append(resolved, r)
		}
	}
	if len(errs) > 0 {
		sort.Slice(errs, func(i, j int) bool { return errs[i].Error() < errs[j].Error() })
		return nil, errors.Join(errs...)
	}
	if len(resolved) == 0 {
		return nil, noInjectorsError(mode, patterns)
	}

	byPkg := make(map[string][]*ResolvedInjector)
	for _, r := range resolved {
		byPkg[r.Injector.PkgPath] = append(byPkg[r.Injector.PkgPath], r)
	}
	pkgPaths := make([]string, 0, len(byPkg))
	for p := range byPkg {
		pkgPaths = append(pkgPaths, p)
	}
	sort.Strings(pkgPaths)

	res := &GenResult{Graph: BuildGraph(resolved, modulePath, now())}
	for _, p := range pkgPaths {
		injs := byPkg[p]
		src, err := GenerateFile(mode, injs)
		if err != nil {
			return nil, err
		}
		dir := filepath.Dir(injs[0].Injector.FilePath)
		res.Files = append(res.Files, GeneratedFile{Path: filepath.Join(dir, outFile), Content: src})
	}
	sort.Slice(res.Files, func(i, j int) bool { return res.Files[i].Path < res.Files[j].Path })
	return res, nil
}

// Write writes every generated file and, when graphPath is not empty, the
// dependency graph. Files whose content is unchanged are left untouched.
func (r *GenResult) Write(graphPath string) error {
	for _, f := range r.Files {
		if old, err := os.ReadFile(f.Path); err == nil && bytes.Equal(old, f.Content) {
			continue
		}
		if err := os.WriteFile(f.Path, f.Content, 0o644); err != nil { //nolint:gosec // generated source, readable like any source file
			return fmt.Errorf("writing generated file: %w", err)
		}
	}
	if graphPath == "" {
		return nil
	}
	return graphschema.WriteFile(graphPath, r.Graph)
}

// noInjectorsError explains where injectors were looked for and points at
// the other modes, since running the wrong mode is the usual cause.
func noInjectorsError(mode Mode, patterns []string) error {
	var others []string
	for _, m := range Modes {
		if m.Name != mode.Name {
			others = append(others, fmt.Sprintf("-mode=%s for %s.Build", m.Name, m.MarkerPkgPath))
		}
	}
	return fmt.Errorf("no injectors found in %v (mode %s: looking for %s.Build in files tagged %s; use %s)",
		patterns, mode.Name, mode.MarkerPkgPath, mode.InjectTag, strings.Join(others, ", "))
}
