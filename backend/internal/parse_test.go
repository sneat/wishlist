package internal

import "testing"

func TestParseIntOrZero(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"   ", 0},
		{"0", 0},
		{"7", 7},
		{"2019", 2019},
		{"-3", -3},
		{"not a number", 0},
		{"7.5", 0},
		{"  42  ", 42},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := parseIntOrZero(tc.in); got != tc.want {
				t.Fatalf("parseIntOrZero(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseFloatOrZero(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"", 0},
		{"0", 0},
		{"7.85", 7.85},
		{"7", 7},
		{"-1.5", -1.5},
		{"not a number", 0},
		{"  3.14  ", 3.14},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := parseFloatOrZero(tc.in); got != tc.want {
				t.Fatalf("parseFloatOrZero(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
