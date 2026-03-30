package handlers

import "testing"

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "strips HTML tags",
			input: `<script>alert("xss")</script>like`,
			want:  `alert("xss")like`,
		},
		{
			name:  "strips nested tags",
			input: `<div><b>bold</b></div>`,
			want:  "bold",
		},
		{
			name:  "trims whitespace",
			input: "  like  ",
			want:  "like",
		},
		{
			name:  "passes clean strings through",
			input: "11111111-1111-1111-1111-111111111111",
			want:  "11111111-1111-1111-1111-111111111111",
		},
		{
			name:  "handles empty string",
			input: "",
			want:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeString(tc.input)
			if got != tc.want {
				t.Errorf("sanitizeString(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
