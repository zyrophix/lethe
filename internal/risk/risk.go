package risk

import (
	"fmt"
	"strings"
)

type RiskLevel int

const (
	RiskSafe RiskLevel = iota
	RiskRisky
	RiskDestructive
)

func (r RiskLevel) String() string {
	switch r {
	case RiskSafe:
		return "safe"
	case RiskRisky:
		return "risky"
	case RiskDestructive:
		return "destructive"
	default:
		return "unknown"
	}
}

func (r RiskLevel) Color() string {
	switch r {
	case RiskSafe:
		return "\033[32m"
	case RiskRisky:
		return "\033[33m"
	case RiskDestructive:
		return "\033[31m"
	default:
		return ""
	}
}

func (r RiskLevel) Reset() string {
	return "\033[0m"
}

func ParseRiskLevel(s string) (RiskLevel, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "safe":
		return RiskSafe, nil
	case "risky":
		return RiskRisky, nil
	case "destructive":
		return RiskDestructive, nil
	default:
		return RiskSafe, fmt.Errorf("invalid risk level: %q (must be safe, risky, or destructive)", s)
	}
}

type Policy struct {
	MaxRisk      RiskLevel
	AllowDestroy bool
}

func NewPolicy(maxRisk RiskLevel) Policy {
	return Policy{
		MaxRisk:      maxRisk,
		AllowDestroy: maxRisk >= RiskDestructive,
	}
}

func (p Policy) Allowed(level RiskLevel) bool {
	return level <= p.MaxRisk
}
