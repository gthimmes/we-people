package iam

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Acme Corp":            "acme-corp",
		"  Spaces  Everywhere ": "spaces-everywhere",
		"Weird!!!Chars@@@Here":  "weird-chars-here",
		"UPPER":                 "upper",
		"multi---dash":          "multi-dash",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}
