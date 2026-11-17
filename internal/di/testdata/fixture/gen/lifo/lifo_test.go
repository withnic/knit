package lifo

import (
	"reflect"
	"testing"
)

// This test only compiles after knit has generated wire_gen.go; it is run by
// knit's end-to-end test against a temporary copy of the fixture.
func TestInitializeRoot_LIFO(t *testing.T) {
	tests := []struct {
		failAt  string
		wantErr bool
		want    []string
	}{
		{failAt: "", want: []string{"open A", "open B", "open C", "close C", "close B", "close A"}},
		{failAt: "C", wantErr: true, want: []string{"open A", "open B", "close B", "close A"}},
		{failAt: "A", wantErr: true, want: nil},
	}
	for _, tt := range tests {
		t.Run("fail="+tt.failAt, func(t *testing.T) {
			Log, FailAt = nil, tt.failAt
			root, cleanup, err := InitializeRoot()
			if tt.wantErr {
				if err == nil || root != nil || cleanup != nil {
					t.Fatalf("InitializeRoot() = %v, cleanup=%t, %v; want nil, nil, error", root, cleanup != nil, err)
				}
			} else {
				if err != nil || root == nil || root.C == nil {
					t.Fatalf("InitializeRoot() = %v, _, %v; want root, nil", root, err)
				}
				cleanup()
			}
			if !reflect.DeepEqual(Log, tt.want) {
				t.Errorf("Log = %q, want %q", Log, tt.want)
			}
		})
	}
}
