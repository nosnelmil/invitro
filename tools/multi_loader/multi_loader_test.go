package multi_loader

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	multiLoaderTestConfigPath string
	configPath string
)

func initLogger() *log.Logger {
	logger := log.New()
	logger.SetFormatter(&log.TextFormatter{
		TimestampFormat: time.StampMilli,
		FullTimestamp:   true,
	})
	logger.SetOutput(os.Stdout)
	logger.SetLevel(log.InfoLevel)
	return logger
}

func init() {
	wd, _ := os.Getwd()
	multiLoaderTestConfigPath = filepath.Join(wd, "./test_multi_loader_config.json")
	configPath = filepath.Join(wd, "./base_loader_config.json")
	log.Info("Test config path: ", multiLoaderTestConfigPath)
	log.Info("Test config path: ", configPath)
}

// Test multi-loader unpack experiment
func TestUnpackExperiment(t *testing.T) {
	logger := initLogger()

	// Create a new multi-loader driver with the test config path
	multiLoader, err := NewMultiLoaderDriver(multiLoaderTestConfigPath, logger, "info", false, false)
	if err != nil {
		t.Fatalf("Failed to create multi-loader driver: %v", err)
	}

	t.Run("Unpack using TracesDir", func(t *testing.T) {
		for _, experiment := range multiLoader.MultiLoaderConfig.Experiments {
			// Unpack experiment
			subExperiments := multiLoader.unpackExperiment(experiment)
			
			// Check if the number of sub-experiments matches expected count
			expectedSubExperiments := len(experiment.TraceValues)
			if len(subExperiments) != expectedSubExperiments {
				t.Errorf("Expected %d sub-experiments, got %d", expectedSubExperiments, len(subExperiments))
			}

			// Validate each unpacked sub-experiment's configurations
			for i, subExp := range subExperiments {
				// Check name format (assuming each sub-experiment is named based on the main experiment's name and trace value)
				expectedName := experiment.Name + "_" + experiment.TraceValues[i].(string)
				if subExp.Name != expectedName {
					t.Errorf("Expected sub-experiment name '%s', got '%s'", expectedName, subExp.Name)
				}

				// Check core configurations
				if subExp.Config["ExperimentDuration"] != experiment.Config["ExperimentDuration"] {
					t.Errorf("Expected ExperimentDuration %v, got %v", experiment.Config["ExperimentDuration"], subExp.Config["ExperimentDuration"])
				}
				if subExp.Config["Platform"] != experiment.Config["Platform"] {
					t.Errorf("Expected Platform %v, got %v", experiment.Config["Platform"], subExp.Config["Platform"])
				}

				// Validate TracesDir, TracesFormat, and OutputDir
				if subExp.TracesFormat != experiment.TracesFormat {
					t.Errorf("Expected TracesFormat '%s', got '%s'", experiment.TracesFormat, subExp.TracesFormat)
				}
				if subExp.OutputDir != experiment.OutputDir {
					t.Errorf("Expected OutputDir '%s', got '%s'", experiment.OutputDir, subExp.OutputDir)
				}
				
				// Optional: check other configurations like IatGeneration and Generated if required
				if subExp.IatGeneration != experiment.IatGeneration {
					t.Errorf("Expected IatGeneration %v, got %v", experiment.IatGeneration, subExp.IatGeneration)
				}
				if subExp.Generated != experiment.Generated {
					t.Errorf("Expected Generated %v, got %v", experiment.Generated, subExp.Generated)
				}
			}
		}
	})
}


// Unpack using TracesDir
// Unpack using Trace Format
// Unpack single experiment
//  Expected output a list of subexperiments with the correct configs

// Test prepare experiment with a subexperiment
// output directory should be created
// check temp experiment config (to be passed to loader)
// make sure it merged correctly with base loader config

// Test mergeConfigurations method
// Check if merging with base configuration is correct

