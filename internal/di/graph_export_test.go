package di

import (
	"reflect"
	"testing"
	"time"

	"github.com/withnic/knit/internal/shared/graphschema"
)

func TestBuildGraph(t *testing.T) {
	const p = fixtureMod + "/basic"
	basic, _ := resolveFixture(t, "basic")
	samename, _ := resolveFixture(t, "samename/di")
	at := time.Date(2026, 11, 17, 21, 0, 0, 0, time.FixedZone("JST", 9*60*60))

	// Pass injectors out of order: the graph must be sorted regardless.
	got := BuildGraph(append(samename, basic...), fixtureMod, at)

	if got.Version != graphschema.Version || got.GeneratedAt != "2026-11-17T21:00:00+09:00" {
		t.Errorf("header = (%q, %q), want (%q, 2026-11-17T21:00:00+09:00)", got.Version, got.GeneratedAt, graphschema.Version)
	}
	if len(got.Injectors) != 2 {
		t.Fatalf("got %d injectors, want 2", len(got.Injectors))
	}
	want := graphschema.Injector{
		Name:     "InitializeServer",
		PkgPath:  p,
		RootType: "*" + p + ".Server",
		Dependencies: []graphschema.Dependency{
			{PkgPath: p, PkgDir: "./basic", Type: "*" + p + ".DB", Provider: p + ".NewDB"},
			{PkgPath: p, PkgDir: "./basic", Type: "*" + p + ".Server", Provider: p + ".NewServer"},
			{PkgPath: p, PkgDir: "./basic", Type: "*" + p + ".sqlStore", Provider: p + ".NewSQLStore"},
			{PkgPath: p, PkgDir: "./basic", Type: p + ".Store", Provider: p + ".NewSQLStore", BoundFrom: "*" + p + ".sqlStore"},
		},
	}
	if !reflect.DeepEqual(got.Injectors[0], want) {
		t.Errorf("Injectors[0] =\n%+v\nwant\n%+v", got.Injectors[0], want)
	}
	if got.Injectors[1].PkgPath != fixtureMod+"/samename/di" || len(got.Injectors[1].Dependencies) != 3 {
		t.Errorf("Injectors[1] = %+v, want samename/di with 3 dependencies", got.Injectors[1])
	}
}

func TestBuildGraph_Empty(t *testing.T) {
	data, err := graphschema.Marshal(BuildGraph(nil, "", time.Unix(0, 0).UTC()))
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"version\": \"1.0\",\n  \"generated_at\": \"1970-01-01T00:00:00Z\",\n  \"injectors\": []\n}\n"
	if string(data) != want {
		t.Errorf("Marshal() =\n%s\nwant\n%s", data, want)
	}
}

func TestPkgDir(t *testing.T) {
	tests := []struct {
		mod, pkg, want string
	}{
		{mod: "github.com/org/repo", pkg: "github.com/org/repo", want: "."},
		{mod: "github.com/org/repo", pkg: "github.com/org/repo/internal/handler", want: "./internal/handler"},
		{mod: "github.com/org/repo", pkg: "github.com/org/repository/x", want: ""},
		{mod: "github.com/org/repo", pkg: "context", want: ""},
		{mod: "", pkg: "github.com/org/repo/x", want: ""},
	}
	for _, tt := range tests {
		if got := PkgDir(tt.mod, tt.pkg); got != tt.want {
			t.Errorf("PkgDir(%q, %q) = %q, want %q", tt.mod, tt.pkg, got, tt.want)
		}
	}
}
