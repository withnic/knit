package app

import (
	"context"
	"reflect"
	"testing"
)

// This test only compiles after knit has generated knit_gen.go; it is run by
// knit's end-to-end test against a temporary copy of the fixture.
func TestInitializeServer(t *testing.T) {
	Log = nil
	s, cleanup, err := InitializeServer(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Store.Get(); got != "default" {
		t.Errorf("Get() = %q", got)
	}
	cleanup()
	if want := []string{"open", "close"}; !reflect.DeepEqual(Log, want) {
		t.Errorf("Log = %v, want %v", Log, want)
	}
}
