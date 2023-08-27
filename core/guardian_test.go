package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/axiomesh/guardian/repo"
	"github.com/stretchr/testify/assert"
)

func TestStart(t *testing.T) {
	tempDir := t.TempDir()

	t.Logf("temp dir: %s", tempDir)

	dir, err := os.Getwd()
	assert.Nil(t, err)

	c := repo.DefaultConfig(tempDir)
	c.AxiomPath = filepath.Join(dir, "test_axiom")
	c.Log.Level = "debug"

	guardian, err := NewGuardian(context.Background(), c, &MockClient{})
	assert.Nil(t, err)

	err = guardian.Start()
	assert.Nil(t, err)

	time.Sleep(5 * time.Second)
}
