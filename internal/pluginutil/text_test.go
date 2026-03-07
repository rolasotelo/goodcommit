package pluginutil

import "testing"

func TestUppercaseFirstRuneASCII(t *testing.T) {
	if got := UppercaseFirstRune("hello"); got != "Hello" {
		t.Fatalf("UppercaseFirstRune() = %q, want %q", got, "Hello")
	}
}

func TestUppercaseFirstRuneUTF8(t *testing.T) {
	if got := UppercaseFirstRune("áccent"); got != "Áccent" {
		t.Fatalf("UppercaseFirstRune() = %q, want %q", got, "Áccent")
	}
}

func TestLowercaseFirstRuneASCII(t *testing.T) {
	if got := LowercaseFirstRune("Hello"); got != "hello" {
		t.Fatalf("LowercaseFirstRune() = %q, want %q", got, "hello")
	}
}

func TestLowercaseFirstRuneUTF8(t *testing.T) {
	if got := LowercaseFirstRune("Ñandú"); got != "ñandú" {
		t.Fatalf("LowercaseFirstRune() = %q, want %q", got, "ñandú")
	}
}

func TestCaseHelpersPreserveInvalidUTF8Prefix(t *testing.T) {
	raw := string([]byte{0xff, 'A', 'B'})
	if got := UppercaseFirstRune(raw); got != raw {
		t.Fatalf("UppercaseFirstRune() changed invalid input: got %q want %q", got, raw)
	}
	if got := LowercaseFirstRune(raw); got != raw {
		t.Fatalf("LowercaseFirstRune() changed invalid input: got %q want %q", got, raw)
	}
}
