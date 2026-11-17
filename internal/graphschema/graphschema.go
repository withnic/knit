// Package graphschema defines the .knit/di-graph.json format written by
// knit gen.
//
// The file records which providers and types every injector depends on. It
// is a stable, versioned output for external tools (e.g. test impact
// analysis); see docs/02_data_models.md §2 for the specification.
package graphschema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Version is the schema version written to and accepted from di-graph.json.
const Version = "1.0"

// DefaultPath is the default location of the graph file relative to the
// module root.
const DefaultPath = ".knit/di-graph.json"

// Graph is the root document of di-graph.json.
type Graph struct {
	// Version is the schema version; always Version when written by knit.
	Version string `json:"version"`
	// GeneratedAt is the RFC 3339 generation timestamp.
	GeneratedAt string `json:"generated_at"`
	// Injectors lists every generated injector, sorted by PkgPath then Name.
	Injectors []Injector `json:"injectors"`
}

// Injector is the dependency subgraph of one injector function.
type Injector struct {
	// Name is the injector function name, e.g. InitializeUserHandler.
	Name string `json:"name"`
	// PkgPath is the import path of the package declaring the injector.
	PkgPath string `json:"pkg_path"`
	// RootType is the fully qualified type the injector returns.
	RootType string `json:"root_type"`
	// Dependencies lists every type built for the injector, sorted by Type
	// then Provider.
	Dependencies []Dependency `json:"dependencies"`
}

// Dependency is one type supplied while building an injector.
type Dependency struct {
	// PkgPath is the import path of the provider's package.
	PkgPath string `json:"pkg_path"`
	// PkgDir is the provider package directory relative to the module root
	// (e.g. "./internal/handler"), or empty for packages outside the module.
	PkgDir string `json:"pkg_dir"`
	// Type is the fully qualified type supplied to the graph.
	Type string `json:"type"`
	// Provider is the fully qualified provider function, e.g.
	// github.com/org/repo/internal/handler.NewUserHandler.
	Provider string `json:"provider"`
	// BoundFrom is the concrete type Type was bound from via wire.Bind, or
	// empty when Type is provided directly.
	BoundFrom string `json:"bound_from,omitempty"`
}

// Marshal encodes g as indented JSON terminated by a newline. The output is
// deterministic for a given g.
func Marshal(g *Graph) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(g); err != nil {
		return nil, fmt.Errorf("encoding di-graph: %w", err)
	}
	return buf.Bytes(), nil
}

// Decode reads a graph from r and validates its version.
func Decode(r io.Reader) (*Graph, error) {
	var g Graph
	dec := json.NewDecoder(r)
	if err := dec.Decode(&g); err != nil {
		return nil, fmt.Errorf("decoding di-graph: %w", err)
	}
	if g.Version != Version {
		return nil, fmt.Errorf("decoding di-graph: unsupported version %q (want %q)", g.Version, Version)
	}
	return &g, nil
}

// ReadFile reads and validates the graph stored at path.
func ReadFile(path string) (*Graph, error) {
	f, err := os.Open(path) //nolint:gosec // path is supplied by the user on purpose
	if err != nil {
		return nil, fmt.Errorf("reading di-graph: %w", err)
	}
	defer f.Close()
	return Decode(f)
}

// WriteFile writes g to path, creating parent directories as needed.
func WriteFile(path string, g *Graph) error {
	data, err := Marshal(g)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // build output directory, like any source tree directory
		return fmt.Errorf("writing di-graph: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil { //nolint:gosec // build output, readable like any source file
		return fmt.Errorf("writing di-graph: %w", err)
	}
	return nil
}
