package driver

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/vhive-serverless/loader/pkg/config"
)

// Mock logger helper
func getMockLogger() *logrus.Logger {
	logger := logrus.New()
	logger.Out = os.Stdout
	return logger
}

// Test for NewMultiLoaderDriver
func TestNewMultiLoaderDriver(t *testing.T) {
	logger := getMockLogger()
	configPath := "pkg/config/test_multi_loader_config.json"
	driver, err := NewMultiLoaderDriver(configPath, logger, "info", false, false)

	assert.Nil(t, err, "Expected no error when creating MultiLoaderDriver")
	assert.NotNil(t, driver, "Expected driver to be initialized")
}

// Test for RunDryRun
func TestRunDryRun(t *testing.T) {
	driver := &MultiLoaderDriver{
		Logger:    getMockLogger(),
		DryRun:    false,
	}

	driver.RunDryRun()
	assert.True(t, driver.DryRun, "Expected DryRun to be set to true after calling RunDryRun")
}

// Test for unpackSingleExperiment
func TestUnpackSingleExperiment(t *testing.T) {
	driver := &MultiLoaderDriver{
		Logger: getMockLogger(),
	}

	experiment := config.LoaderExperiment{
		Name: "testExperiment",
		Config: map[string]interface{}{"OutputPathPrefix": "/tmp/output"},
	}

	result := driver.unpackSingleExperiment(experiment)
	assert.Len(t, result, 1, "Expected single experiment unpacked")
	assert.Equal(t, "testExperiment", result[0].Name, "Expected unpacked experiment to have the same name")
}