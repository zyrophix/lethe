package risk

import "testing"

func TestRiskLevelString(t *testing.T) {
	tests := []struct {
		level    RiskLevel
		expected string
	}{
		{RiskSafe, "safe"},
		{RiskRisky, "risky"},
		{RiskDestructive, "destructive"},
	}
	for _, tt := range tests {
		if got := tt.level.String(); got != tt.expected {
			t.Errorf("RiskLevel(%d).String() = %q, want %q", tt.level, got, tt.expected)
		}
	}
}

func TestParseRiskLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected RiskLevel
		wantErr  bool
	}{
		{"safe", RiskSafe, false},
		{"risky", RiskRisky, false},
		{"destructive", RiskDestructive, false},
		{"SAFE", RiskSafe, false},
		{"Risky", RiskRisky, false},
		{"  safe  ", RiskSafe, false},
		{"invalid", RiskSafe, true},
		{"", RiskSafe, true},
	}
	for _, tt := range tests {
		got, err := ParseRiskLevel(tt.input)
		if tt.wantErr && err == nil {
			t.Errorf("ParseRiskLevel(%q): expected error, got none", tt.input)
		}
		if !tt.wantErr && got != tt.expected {
			t.Errorf("ParseRiskLevel(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestPolicyAllowed(t *testing.T) {
	tests := []struct {
		maxRisk  RiskLevel
		check    RiskLevel
		expected bool
	}{
		{RiskSafe, RiskSafe, true},
		{RiskSafe, RiskRisky, false},
		{RiskSafe, RiskDestructive, false},
		{RiskRisky, RiskSafe, true},
		{RiskRisky, RiskRisky, true},
		{RiskRisky, RiskDestructive, false},
		{RiskDestructive, RiskSafe, true},
		{RiskDestructive, RiskRisky, true},
		{RiskDestructive, RiskDestructive, true},
	}
	for _, tt := range tests {
		p := NewPolicy(tt.maxRisk)
		if got := p.Allowed(tt.check); got != tt.expected {
			t.Errorf("Policy{max=%d}.Allowed(%d) = %v, want %v", tt.maxRisk, tt.check, got, tt.expected)
		}
	}
}
