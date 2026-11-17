package di

import (
	"fmt"
	"strings"
)

// Mode selects the injector declaration dialect: which marker package
// declares injectors, which build tag guards injector files and which file
// the generated code is written to.
type Mode struct {
	// Name is the mode name used on the command line and in messages; it is
	// also the conventional name of the marker package ("knit.Build").
	Name string
	// MarkerPkgPath is the import path of the package whose Build / NewSet /
	// Bind calls declare injectors.
	MarkerPkgPath string
	// InjectTag is the build tag that enables injector declaration files. The
	// generated file is guarded by its negation.
	InjectTag string
	// OutFile is the default name of the generated file.
	OutFile string
}

var (
	// KnitMode is knit's native dialect and the default: knit.Build etc. from
	// github.com/withnic/knit, //go:build knitinject, knit_gen.go.
	KnitMode = Mode{
		Name:          "knit",
		MarkerPkgPath: "github.com/withnic/knit",
		InjectTag:     "knitinject",
		OutFile:       "knit_gen.go",
	}
	// WireMode reads and writes exactly what google/wire does: wire.Build etc.
	// from github.com/google/wire, //go:build wireinject, wire_gen.go.
	WireMode = Mode{
		Name:          "wire",
		MarkerPkgPath: "github.com/google/wire",
		InjectTag:     "wireinject",
		OutFile:       "wire_gen.go",
	}
)

// Modes lists the supported modes, default first.
var Modes = []Mode{KnitMode, WireMode}

// ParseMode returns the mode called name.
func ParseMode(name string) (Mode, error) {
	names := make([]string, len(Modes))
	for i, m := range Modes {
		if m.Name == name {
			return m, nil
		}
		names[i] = m.Name
	}
	return Mode{}, fmt.Errorf("unknown mode %q (want one of %s)", name, strings.Join(names, ", "))
}

// orDefault returns m, or KnitMode for the zero Mode.
func (m Mode) orDefault() Mode {
	if m == (Mode{}) {
		return KnitMode
	}
	return m
}
