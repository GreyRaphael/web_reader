package terminal

import (
	"testing"
)

func TestIsControlMessage(t *testing.T) {
	tests := []struct {
		name string
		data string
		want bool
	}{
		{"empty", "", false},
		{"plain text", "ls -la\n", false},
		{"resize json", `{"type":"resize","cols":80,"rows":24}`, true},
		{"invalid json brace", `{"incomplete`, false},
		{"json without type", `{"hello":"world"}`, false},
		{"binary data", "\x1b[31m", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isControlMessage([]byte(tt.data)); got != tt.want {
				t.Fatalf("isControlMessage(%q) = %v, want %v", tt.data, got, tt.want)
			}
		})
	}
}
