package main

import "testing"

func TestClampShellExpandedWidth(t *testing.T) {
	tests := []struct {
		name  string
		in    int
		want  int
	}{
		{name: "negative", in: -1, want: shellExpandedWidth},
		{name: "zero", in: 0, want: shellExpandedWidth},
		{name: "below cap", in: shellExpandedWidth - 1, want: shellExpandedWidth - 1},
		{name: "at cap", in: shellExpandedWidth, want: shellExpandedWidth},
		{name: "above cap", in: shellExpandedWidth + 42, want: shellExpandedWidth},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampShellExpandedWidth(tt.in); got != tt.want {
				t.Fatalf("clampShellExpandedWidth(%d) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}
