package lifo

import (
	"reflect"
	"testing"
)

// This test only compiles after knit has generated wire_gen.go; it is run by
// knit's end-to-end test against a temporary copy of the fixture.
func TestInitializeRoot_LIFO(t *testing.T) {
	all := []string{"open A", "open B", "open C"}
	tests := []struct {
		name                          string
		failAt, nilCleanupAt, panicAt string
		wantErr, wantPanic            bool
		want                          []string
	}{
		{name: "success", want: append(all[:3:3], "close C", "close B", "close A")},
		{name: "last provider fails", failAt: "C", wantErr: true, want: []string{"open A", "open B", "close B", "close A"}},
		{name: "first provider fails", failAt: "A", wantErr: true},
		{name: "nil cleanup is skipped", nilCleanupAt: "B", want: append(all[:3:3], "close C", "close A")},
		{name: "panic does not skip the rest", panicAt: "B", wantPanic: true, want: append(all[:3:3], "close C", "close B", "close A")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Log, FailAt, NilCleanupAt, PanicAt = nil, tt.failAt, tt.nilCleanupAt, tt.panicAt
			root, cleanup, err := InitializeRoot()
			if tt.wantErr {
				if err == nil || root != nil || cleanup != nil {
					t.Fatalf("InitializeRoot() = %v, cleanup=%t, %v; want nil, nil, error", root, cleanup != nil, err)
				}
			} else {
				if err != nil || root == nil || root.C == nil {
					t.Fatalf("InitializeRoot() = %v, _, %v; want root, nil", root, err)
				}
				func() {
					defer func() {
						if r := recover(); (r != nil) != tt.wantPanic {
							t.Errorf("cleanup panic = %v, want panic: %t", r, tt.wantPanic)
						}
					}()
					cleanup()
				}()
			}
			if !reflect.DeepEqual(Log, tt.want) {
				t.Errorf("Log = %q, want %q", Log, tt.want)
			}
		})
	}
}
