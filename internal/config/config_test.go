package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadCfg(t *testing.T) {

	t.Run("test with a valid config file", func(t *testing.T) {
		_, err := LoadCfg("testdata/valid_config.yaml")
		assert.NoError(t, err, "expected no error when loading valid config")

	})

	t.Run("test with an invalid config file", func(t *testing.T) {
		_, err := LoadCfg("testdata/invalid_config.yaml")
		assert.Error(t, err)

	})
	t.Run("Test with a non-existent config file", func(t *testing.T) {
		_, err := LoadCfg("testdata/non_existent.yaml")
		assert.Error(t, err)

	})

}
