package repo

import "testing"

func TestValidateNonConcurrentPatterns(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		wantErr  bool
	}{
		{name: "nil", patterns: nil, wantErr: false},
		{name: "valid", patterns: []string{"gcp/confluent/**", "aws/heavy-stack"}, wantErr: false},
		{name: "empty", patterns: []string{"gcp/**", ""}, wantErr: true},
		{name: "whitespace", patterns: []string{"  "}, wantErr: true},
		{name: "malformed", patterns: []string{"["}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNonConcurrentPatterns(tt.patterns)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateNonConcurrentPatterns(%q) error = %v, wantErr %v", tt.patterns, err, tt.wantErr)
			}
		})
	}
}
