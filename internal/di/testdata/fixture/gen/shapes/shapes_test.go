package shapes

import (
	"reflect"
	"testing"
)

// This test only compiles after knit has generated wire_gen.go.
func TestShapes(t *testing.T) {
	if InitializeValue() == nil {
		t.Error("InitializeValue() = nil")
	}
	if v, err := InitializeErr(); v == nil || err != nil {
		t.Errorf("InitializeErr() = %v, %v", v, err)
	}

	Log = nil
	_, cleanup := InitializeClean()
	cleanup()
	if want := []string{"clean"}; !reflect.DeepEqual(Log, want) {
		t.Errorf("after InitializeClean cleanup: Log = %q, want %q", Log, want)
	}

	Log = nil
	root, cleanup, err := InitializeRoot()
	if root == nil || err != nil {
		t.Fatalf("InitializeRoot() = %v, _, %v", root, err)
	}
	cleanup()
	if want := []string{"both", "clean"}; !reflect.DeepEqual(Log, want) {
		t.Errorf("after InitializeRoot cleanup: Log = %q, want %q", Log, want)
	}

	v, cleanup, err := InitializeNoCleanup()
	if v == nil || cleanup == nil || err != nil {
		t.Fatalf("InitializeNoCleanup() = %v, cleanup=%t, %v", v, cleanup != nil, err)
	}
	cleanup()
}
