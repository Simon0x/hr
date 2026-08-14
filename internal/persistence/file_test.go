package persistence_test

import (
	"testing"

	"github.com/Simon0x/hr/internal/persistence"
	"github.com/Simon0x/hr/internal/persistence/storetest"
)

func TestFile(t *testing.T) {
	storetest.Run(t, func(t *testing.T) persistence.Store {
		return persistence.File{Root: t.TempDir()}
	})
}
