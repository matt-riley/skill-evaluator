package main

import "testing"

func TestRunnerExtractTokens(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   int
	}{
		{
			name:   "json total_tokens number",
			output: `{"usage": {"total_tokens": 123}}`,
			want:   123,
		},
		{
			name:   "plain total tokens colon",
			output: "total tokens: 456",
			want:   456,
		},
		{
			name:   "plain tokens colon",
			output: "tokens: 789 extra",
			want:   789,
		},
		{
			name:   "input_tokens",
			output: "input_tokens: 321",
			want:   321,
		},
		{
			name:   "no match returns zero",
			output: "no token data here",
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractTokens(tt.output); got != tt.want {
				t.Errorf("extractTokens(%q) = %d, want %d", tt.output, got, tt.want)
			}
		})
	}
}
