package main

import "testing"

func TestRunnerExtractPiTokens(t *testing.T) {
	// Two assistant turns: 12744 + 500 = 13244. message_end fires once per
	// assistant message; usage also repeats in message_update/turn_end/agent_end
	// and must NOT be double-counted.
	stream := `{"type":"session","version":3}` + "\n" +
		`{"type":"agent_start"}` + "\n" +
		`{"type":"message_end","message":{"role":"user"}}` + "\n" +
		`{"type":"message_update","message":{"role":"assistant","usage":{"totalTokens":12744}}}` + "\n" +
		`{"type":"message_end","message":{"role":"assistant","usage":{"totalTokens":12744}}}` + "\n" +
		`{"type":"turn_end","message":{"role":"assistant","usage":{"totalTokens":12744}}}` + "\n" +
		`{"type":"message_end","message":{"role":"assistant","usage":{"totalTokens":500}}}` + "\n" +
		`{"type":"agent_end","messages":[{"role":"assistant","usage":{"totalTokens":12744}}]}` + "\n"
	if got := extractPiTokens(stream); got != 13244 {
		t.Errorf("extractPiTokens = %d, want 13244", got)
	}
	// Non-pi agent falls back to the regex heuristic.
	if got := tokensFromOutput("claude", "total tokens: 42"); got != 42 {
		t.Errorf("tokensFromOutput(claude, ...) = %d, want 42", got)
	}
	// pi with no usage data returns 0.
	if got := extractPiTokens("not json at all"); got != 0 {
		t.Errorf("extractPiTokens(no data) = %d, want 0", got)
	}
}

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
