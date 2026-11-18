package di

import (
	"go/token"
	"go/types"
	"testing"
)

func TestNewTypeKey(t *testing.T) {
	pkgA := types.NewPackage("github.com/org/repo/internal/pkga", "pkga")
	pkgB := types.NewPackage("github.com/org/repo/internal/pkgb", "pkgb")
	named := func(pkg *types.Package, name string, under types.Type) *types.Named {
		return types.NewNamed(types.NewTypeName(token.NoPos, pkg, name, nil), under, nil)
	}
	configA := named(pkgA, "Config", types.NewStruct(nil, nil))
	configB := named(pkgB, "Config", types.NewStruct(nil, nil))
	alias := types.NewAlias(types.NewTypeName(token.NoPos, pkgB, "ConfigAlias", nil), configA)

	tests := []struct {
		name string
		typ  types.Type
		want TypeKey
	}{
		{"value type", configA, "github.com/org/repo/internal/pkga.Config"},
		{"pointer type", types.NewPointer(configA), "*github.com/org/repo/internal/pkga.Config"},
		{"same name in other package", configB, "github.com/org/repo/internal/pkgb.Config"},
		{"pointer to same name in other package", types.NewPointer(configB), "*github.com/org/repo/internal/pkgb.Config"},
		{"slice", types.NewSlice(configA), "[]github.com/org/repo/internal/pkga.Config"},
		{"map", types.NewMap(types.Typ[types.String], types.NewPointer(configB)), "map[string]*github.com/org/repo/internal/pkgb.Config"},
		{"basic", types.Typ[types.Int], "int"},
		{"pointer to alias", types.NewPointer(alias), "*github.com/org/repo/internal/pkga.Config"},
		{"slice of alias", types.NewSlice(alias), "[]github.com/org/repo/internal/pkga.Config"},
		{"map of alias", types.NewMap(alias, alias), "map[github.com/org/repo/internal/pkga.Config]github.com/org/repo/internal/pkga.Config"},
		{"alias resolves to target", alias, "github.com/org/repo/internal/pkga.Config"},
		{"error", types.Universe.Lookup("error").Type(), "error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewTypeKey(tt.typ); got != tt.want {
				t.Errorf("NewTypeKey() = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("same name types do not collide", func(t *testing.T) {
		if NewTypeKey(configA) == NewTypeKey(configB) {
			t.Fatalf("pkga.Config and pkgb.Config share key %q", NewTypeKey(configA))
		}
		if NewTypeKey(configA) == NewTypeKey(types.NewPointer(configA)) {
			t.Fatalf("value and pointer types share key %q", NewTypeKey(configA))
		}
	})
}

func TestTypeKey_ShortAndIsPointer(t *testing.T) {
	tests := []struct {
		key       TypeKey
		short     string
		isPointer bool
	}{
		{"*github.com/org/repo/internal/db.Pool", "*db.Pool", true},
		{"github.com/org/repo/internal/service.UserService", "service.UserService", false},
		{"map[string]*github.com/org/repo/a.B", "map[string]*a.B", false},
		{"[]example.com/x/y.Z", "[]y.Z", false},
		{"string", "string", false},
	}
	for _, tt := range tests {
		t.Run(string(tt.key), func(t *testing.T) {
			if got := tt.key.Short(); got != tt.short {
				t.Errorf("Short() = %q, want %q", got, tt.short)
			}
			if got := tt.key.IsPointer(); got != tt.isPointer {
				t.Errorf("IsPointer() = %v, want %v", got, tt.isPointer)
			}
		})
	}
}
