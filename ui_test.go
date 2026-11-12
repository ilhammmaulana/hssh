package main

import (
	"bufio"
	"strings"
	"testing"
)

func TestPromptHandling(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		defaultValue string
		expected     string
	}{
		{
			name:         "Simple input with newline",
			input:        "production\n",
			defaultValue: "default",
			expected:     "production",
		},
		{
			name:         "Input with CRLF",
			input:        "staging\r\n",
			defaultValue: "default",
			expected:     "staging",
		},
		{
			name:         "Empty input uses default",
			input:        "\n",
			defaultValue: "default_val",
			expected:     "default_val",
		},
		{
			name:         "CRLF empty input uses default",
			input:        "\r\n",
			defaultValue: "default_val",
			expected:     "default_val",
		},
		{
			name:         "Remove label with hyphen",
			input:        "-\n",
			defaultValue: "old_label",
			expected:     "-",
		},
		{
			name:         "Whitespace trimmed",
			input:        "   cluster-east   \n",
			defaultValue: "default",
			expected:     "cluster-east",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tc.input))
			result := Prompt(reader, "Test", tc.defaultValue)
			if result != tc.expected {
				t.Errorf("Prompt() = %q, want %q", result, tc.expected)
			}
		})
	}
}
