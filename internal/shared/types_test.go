package shared

import (
	"testing"
)

func TestIsEnvSecretRef(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"valid ref", "env:OPENAI_API_KEY", true},
		{"empty name", "env:", false},
		{"no prefix", "OPENAI_API_KEY", false},
		{"plain text", "plaintext-token", false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsEnvSecretRef(tt.value); got != tt.want {
				t.Errorf("IsEnvSecretRef(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestIsSensitiveRefKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		key  string
		want bool
	}{
		{"authTokenRef", true},
		{"apiKeyRef", true},
		{"modelKeyRef", true},
		{"name", false},
		{"endpointPath", false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.key, func(t *testing.T) {
			t.Parallel()
			if got := IsSensitiveRefKey(tt.key); got != tt.want {
				t.Errorf("IsSensitiveRefKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestMatchesPrimitive(t *testing.T) {
	t.Parallel()
	tests := []struct {
		value any
		typ   string
		want  bool
	}{
		{"hello", "string", true},
		{42, "number", true},
		{true, "boolean", true},
		{"hello", "number", false},
		{nil, "string", false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.typ, func(t *testing.T) {
			t.Parallel()
			if got := MatchesPrimitive(tt.value, tt.typ); got != tt.want {
				t.Errorf("MatchesPrimitive(%v, %q) = %v, want %v", tt.value, tt.typ, got, tt.want)
			}
		})
	}
}
