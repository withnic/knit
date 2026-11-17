package graphschema

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func sample() *Graph {
	return &Graph{
		Version:     Version,
		GeneratedAt: "2026-11-17T21:00:00+09:00",
		Injectors: []Injector{{
			Name:     "InitializeUserHandler",
			PkgPath:  "github.com/org/repo/internal/di",
			RootType: "*github.com/org/repo/internal/handler.UserHandler",
			Dependencies: []Dependency{
				{
					PkgPath:  "github.com/org/repo/internal/handler",
					PkgDir:   "./internal/handler",
					Type:     "*github.com/org/repo/internal/handler.UserHandler",
					Provider: "github.com/org/repo/internal/handler.NewUserHandler",
				},
				{
					PkgPath:   "github.com/org/repo/internal/service",
					PkgDir:    "./internal/service",
					Type:      "github.com/org/repo/internal/service.UserService",
					Provider:  "github.com/org/repo/internal/service.NewUserService",
					BoundFrom: "*github.com/org/repo/internal/service.userServiceImpl",
				},
			},
		}},
	}
}

func TestMarshal_Format(t *testing.T) {
	data, err := Marshal(sample())
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		"\n  \"version\": \"1.0\",\n",
		"\"generated_at\": \"2026-11-17T21:00:00+09:00\"",
		"\"bound_from\": \"*github.com/org/repo/internal/service.userServiceImpl\"",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("Marshal() output lacks %q:\n%s", want, s)
		}
	}
	if strings.Count(s, "bound_from") != 1 {
		t.Errorf("empty bound_from must be omitted:\n%s", s)
	}
	if !strings.HasSuffix(s, "}\n") {
		t.Errorf("Marshal() output must end with a newline")
	}
}

func TestWriteReadFile_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".knit", "di-graph.json")
	if err := WriteFile(path, sample()); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	got, err := ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !reflect.DeepEqual(got, sample()) {
		t.Errorf("round trip = %+v, want %+v", got, sample())
	}
}

func TestDecode_Errors(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{name: "unsupported version", in: `{"version":"2.0","injectors":[]}`, want: `unsupported version "2.0"`},
		{name: "invalid json", in: `{`, want: "decoding di-graph"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decode(strings.NewReader(tt.in))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Decode() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}
