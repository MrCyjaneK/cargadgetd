package rgh

import "testing"

func TestResolvePath(t *testing.T) {
	if got := ResolvePath(""); got != DefaultSO {
		t.Fatalf("empty: %q", got)
	}
	if got := ResolvePath("  embedded "); got != DefaultSO {
		t.Fatalf("embedded: %q", got)
	}
	if got := ResolvePath("reversegearhead"); got != DefaultSO {
		t.Fatalf("alias: %q", got)
	}
	if got := ResolvePath("/tmp/custom.so"); got != "/tmp/custom.so" {
		t.Fatalf("custom: %q", got)
	}
}
