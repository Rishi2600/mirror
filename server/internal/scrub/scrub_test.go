package scrub

import "testing"

func TestText(t *testing.T) {
	tests := []struct {
		in, want string
		removed  Removed
	}{
		{"Reach me at jordan.example@example.com today", "Reach me at today", Removed{Emails: 1}},
		{"Call +1 (555) 010-0199 or 555.010.0101", "Call or", Removed{Phones: 2}},
		{"Profile: https://example.com/jordan and www.example.org", "Profile: and", Removed{Links: 2}},
		{"See linkedin.com/in/jordan-example and github.com/jexample", "See and", Removed{Links: 2}},
		{"Worked 2015 - 2019 and 2019-2022 on Go 1.22", "Worked 2015 - 2019 and 2019-2022 on Go 1.22", Removed{}},
		{"Grew revenue 40% across 3 teams", "Grew revenue 40% across 3 teams", Removed{}},
	}
	for _, tt := range tests {
		got, removed := Text(tt.in)
		if got != tt.want || removed != tt.removed {
			t.Errorf("Text(%q) = %q, %+v; want %q, %+v", tt.in, got, removed, tt.want, tt.removed)
		}
	}
}
