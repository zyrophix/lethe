package platform

import (
	"testing"
)

func TestUserHomesNonRoot(t *testing.T) {
	original := isRoot
	defer func() { isRoot = original }()
	isRoot = func() bool { return false }

	homes := UserHomes("/home/current")
	if len(homes) != 1 || homes[0] != "/home/current" {
		t.Errorf("non-root should return only current home, got %v", homes)
	}
}

func TestShredWarning(t *testing.T) {
	origSSD := SSD
	origCoW := CoW
	defer func() { SSD = origSSD; CoW = origCoW }()

	SSD = func() bool { return true }
	CoW = func() bool { return false }
	if got := ShredWarning(); got == "" {
		t.Error("expected warning for SSD")
	}

	SSD = func() bool { return false }
	CoW = func() bool { return false }
	if got := ShredWarning(); got != "" {
		t.Errorf("expected no warning, got %q", got)
	}
}
