package lmstudio

import "testing"

func TestStripThink(t *testing.T) {
	cases := []struct{ in, want string }{
		{"<think>reasoning here</think>\n\n# Summary\nBody", "# Summary\nBody"},
		{"<think>line1\nline2\nline3</think>Answer", "Answer"},
		{"no think tags at all", "no think tags at all"},
		{"  # Clean already  ", "# Clean already"},
		{"<think>unterminated reasoning with no close\n# leaked", ""},
		{"a<think>x</think>b<think>y</think>final", "final"},
	}
	for _, c := range cases {
		if got := stripThink(c.in); got != c.want {
			t.Errorf("stripThink(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
