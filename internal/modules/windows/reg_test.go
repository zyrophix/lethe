package windows

import (
	"testing"

	"github.com/zyrophix/lethe/internal/module"
)

func TestNewModulesRegistered(t *testing.T) {
	r := module.NewRegistry()
	RegisterAll(r)
	for _, name := range []string{"journal", "pagefile", "advanced"} {
		if _, ok := r.GetForPlatform("windows", name); !ok {
			t.Errorf("module %q not registered", name)
		}
	}
}
