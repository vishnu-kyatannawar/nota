package update

import "testing"

func TestParseVersionAcceptsTagsAndPlainVersions(t *testing.T) {
	for _, in := range []string{"4.1.0", "v4.1.0", " v4.1.0 "} {
		got, err := ParseVersion(in)
		if err != nil {
			t.Fatalf("ParseVersion(%q) returned %v", in, err)
		}
		if want := (Version{4, 1, 0}); got != want {
			t.Errorf("ParseVersion(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseVersionRejectsWhatIsNotAVersion(t *testing.T) {
	// "dev" is what a locally built binary reports; it must never compare.
	for _, in := range []string{"dev", "", "4.1", "4.1.0.1", "4.x.0", "-1.0.0", "v"} {
		if got, err := ParseVersion(in); err == nil {
			t.Errorf("ParseVersion(%q) = %v, want an error", in, got)
		}
	}
}

func TestLessComparesFieldsNumerically(t *testing.T) {
	tests := []struct {
		name       string
		a, b       string
		wantAOlder bool
	}{
		{"patch", "4.1.0", "4.1.1", true},
		{"minor", "4.1.0", "4.2.0", true},
		{"major", "4.1.0", "5.0.0", true},
		{"ten beats nine", "4.9.0", "4.10.0", true},
		{"equal is not older", "4.1.0", "4.1.0", false},
		{"newer is not older", "4.2.0", "4.1.0", false},
		{"major outranks minor", "5.0.0", "4.99.0", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := ParseVersion(tt.a)
			if err != nil {
				t.Fatal(err)
			}
			b, err := ParseVersion(tt.b)
			if err != nil {
				t.Fatal(err)
			}
			if got := a.Less(b); got != tt.wantAOlder {
				t.Errorf("%s.Less(%s) = %v, want %v", tt.a, tt.b, got, tt.wantAOlder)
			}
		})
	}
}

func TestStringRoundTrips(t *testing.T) {
	v, err := ParseVersion("v4.10.2")
	if err != nil {
		t.Fatal(err)
	}
	if got := v.String(); got != "4.10.2" {
		t.Errorf("String() = %q, want %q", got, "4.10.2")
	}
}
