package di

import (
	"sort"
	"strings"
	"time"

	"github.com/withnic/knit/internal/shared/graphschema"
)

// BuildGraph converts resolved injectors into the di-graph.json document
// consumed by tia.
//
// modulePath is the main module path; it is used to derive each provider's
// package directory relative to the module root. Injectors are sorted by
// package path then name, and dependencies by type then provider, so the
// output only depends on its inputs (and generatedAt).
func BuildGraph(injs []*ResolvedInjector, modulePath string, generatedAt time.Time) *graphschema.Graph {
	g := &graphschema.Graph{
		Version:     graphschema.Version,
		GeneratedAt: generatedAt.Format(time.RFC3339),
		Injectors:   make([]graphschema.Injector, 0, len(injs)),
	}
	for _, r := range injs {
		g.Injectors = append(g.Injectors, graphschema.Injector{
			Name:         r.Injector.FuncName,
			PkgPath:      r.Injector.PkgPath,
			RootType:     r.Injector.ReturnType.String(),
			Dependencies: dependencies(r, modulePath),
		})
	}
	sort.Slice(g.Injectors, func(i, j int) bool {
		a, b := g.Injectors[i], g.Injectors[j]
		if a.PkgPath != b.PkgPath {
			return a.PkgPath < b.PkgPath
		}
		return a.Name < b.Name
	})
	return g
}

// dependencies lists every provider step of r plus every interface that was
// satisfied through wire.Bind by a provider step.
func dependencies(r *ResolvedInjector, modulePath string) []graphschema.Dependency {
	deps := make([]graphschema.Dependency, 0, len(r.Steps))
	dep := func(p *ProviderNode, t, boundFrom TypeKey) graphschema.Dependency {
		return graphschema.Dependency{
			PkgPath:   p.PkgPath,
			PkgDir:    PkgDir(modulePath, p.PkgPath),
			Type:      t.String(),
			Provider:  p.ID,
			BoundFrom: boundFrom.String(),
		}
	}
	for i := range r.Steps {
		p := &r.Steps[i].Provider
		deps = append(deps, dep(p, p.OutputType, ""))
	}

	bound := make(map[TypeKey]bool)
	addBound := func(v ValueRef) {
		if v.Kind != FromProvider || v.BoundFrom == "" || bound[v.Type] {
			return
		}
		bound[v.Type] = true
		deps = append(deps, dep(&r.Steps[v.Index].Provider, v.Type, v.BoundFrom))
	}
	for i := range r.Steps {
		for _, a := range r.Steps[i].Args {
			addBound(a)
		}
	}
	addBound(r.Root)

	sort.Slice(deps, func(i, j int) bool {
		if deps[i].Type != deps[j].Type {
			return deps[i].Type < deps[j].Type
		}
		return deps[i].Provider < deps[j].Provider
	})
	return deps
}

// PkgDir returns the directory of pkgPath relative to the root of module
// modulePath in "./sub/dir" form ("." for the module root package), or ""
// when pkgPath is outside the module.
func PkgDir(modulePath, pkgPath string) string {
	switch {
	case modulePath == "":
		return ""
	case pkgPath == modulePath:
		return "."
	case strings.HasPrefix(pkgPath, modulePath+"/"):
		return "./" + strings.TrimPrefix(pkgPath, modulePath+"/")
	default:
		return ""
	}
}
