package multi_loader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/vhive-serverless/loader/pkg/config"
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

	// Set Traces Dir
	multiLoader.MultiLoaderConfig.Experiments[0].TracesDir = "./test_multi_trace"

	t.Run("Unpack using TracesDir", func(t *testing.T) {
		for _, experiment := range multiLoader.MultiLoaderConfig.Experiments {
			// unpack experiment
			subExperiments := multiLoader.unpackExperiment(experiment)
			
			// check number of subexperiments
			if len(subExperiments) != 3 {
				t.Errorf("Expected %d sub-experiments, got %d", 3, len(subExperiments))
			}

			expectedOutputDir := []string{"example_1_test", "example_2_test", "example_3.1_test"}

			// Validate unpacked config
			for i, subExp := range subExperiments {
				// Check name
				if subExp.Name != experiment.Name {
					t.Errorf("Expected subexperiment name '%s', got '%s'", experiment.Name, subExp.Name)
				}
				if outputPathPrefix, ok := subExp.Config["OutputPathPrefix"].(string); !(ok && strings.HasSuffix(outputPathPrefix, expectedOutputDir[i])) {
					t.Errorf("Expected OutputPathPrefix '%s', got '%s'", expectedOutputDir[i], subExp.Config["OutputPathPrefix"])
				}
				// Check core configurations
				if subExp.Config["ExperimentDuration"] != experiment.Config["ExperimentDuration"] {
					t.Errorf("Expected ExperimentDuration %v, got %v", experiment.Config["ExperimentDuration"], subExp.Config["ExperimentDuration"])
				}
				if subExp.OutputDir != experiment.OutputDir {
					t.Errorf("Expected OutputDir '%s', got '%s'", experiment.OutputDir, subExp.OutputDir)
				}
			}
		}
	})
	// Unset tracesdir
	multiLoader.MultiLoaderConfig.Experiments[0].TracesDir = ""
	t.Run("Unpack using TraceFormat and TracesValue", func(t *testing.T) {
		for _, experiment := range multiLoader.MultiLoaderConfig.Experiments {
			// unpack experiment
			subExperiments := multiLoader.unpackExperiment(experiment)
			expectedNumberOfSubExperiments := len(experiment.TraceValues)
			// check number of subexperiments
			if len(subExperiments) != expectedNumberOfSubExperiments {
				t.Errorf("Expected %d sub-experiments, got %d", expectedNumberOfSubExperiments, len(subExperiments))
			}

			// Validate unpacked config
			for i, subExp := range subExperiments {
				// Check name
				expectedName := experiment.Name + "_" + experiment.TraceValues[i].(string)
				if subExp.Name != expectedName {
					t.Errorf("Expected subexperiment name '%s', got '%s'", expectedName, subExp.Name)
				}

				// Check core configurations
				if subExp.Config["ExperimentDuration"] != experiment.Config["ExperimentDuration"] {
					t.Errorf("Expected ExperimentDuration %v, got %v", experiment.Config["ExperimentDuration"], subExp.Config["ExperimentDuration"])
				}
				if subExp.OutputDir != experiment.OutputDir {
					t.Errorf("Expected OutputDir '%s', got '%s'", experiment.OutputDir, subExp.OutputDir)
				}
			}
		}
	})
	// Unset tracePath
	multiLoader.MultiLoaderConfig.Experiments[0].TracesDir = ""
	multiLoader.MultiLoaderConfig.Experiments[0].TracesFormat = ""
	multiLoader.MultiLoaderConfig.Experiments[0].TraceValues = nil
	t.Run("Unpack using tracePath", func(t *testing.T) {
		for _, experiment := range multiLoader.MultiLoaderConfig.Experiments {
			// unpack experiment
			subExperiments := multiLoader.unpackExperiment(experiment)
			expectedNumberOfSubExperiments := 1
			// check number of subexperiments
			if len(subExperiments) != expectedNumberOfSubExperiments {
				t.Errorf("Expected %d sub-experiments, got %d", expectedNumberOfSubExperiments, len(subExperiments))
			}

			// Validate unpacked config
			for _, subExp := range subExperiments {
				// Check name
				expectedName := experiment.Name
				if subExp.Name != expectedName {
					t.Errorf("Expected subexperiment name '%s', got '%s'", expectedName, subExp.Name)
				}

				// Check core configurations
				if subExp.Config["ExperimentDuration"] != experiment.Config["ExperimentDuration"] {
					t.Errorf("Expected ExperimentDuration %v, got %v", experiment.Config["ExperimentDuration"], subExp.Config["ExperimentDuration"])
				}
				if subExp.OutputDir != experiment.OutputDir {
					t.Errorf("Expected OutputDir '%s', got '%s'", experiment.OutputDir, subExp.OutputDir)
				}
			}
		}
	})
}

func TestPrepareExperiment(t *testing.T) {
	logger := initLogger()

	// Create a new multi-loader driver with the test config path
	multiLoader, err := NewMultiLoaderDriver(multiLoaderTestConfigPath, logger, "info", false, false)
	if err != nil {
		t.Fatalf("Failed to create multi-loader driver: %v", err)
	}

	subExperiment := config.LoaderExperiment{
		Name: "example_1",
		Config: map[string]interface{}{
			"ExperimentDuration": 10,
			"TracePath": "./test_multi_trace/example_1_test",
			"OutputPathPrefix": "./test_output/example_1_test",
		},
	}

	if err := os.MkdirAll(filepath.Dir(EXPERIMENT_TEMP_CONFIG_PATH), 0755); err != nil {
		t.Fatalf("Failed to create temp config directory: %v", err)
	}
	multiLoader.prepareExperiment(subExperiment)

	 // Check that the output directory and config file were created
	outputDir := "./test_output"
	tempConfigPath := EXPERIMENT_TEMP_CONFIG_PATH

	// Verify the output directory exists
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		t.Errorf("Expected output directory '%s' to be created, but it was not", outputDir)
	}

		// Verify the temporary config file exists
	if _, err := os.Stat(tempConfigPath); os.IsNotExist(err) {
		t.Errorf("Expected temp config file '%s' to be created, but it was not", tempConfigPath)
	}

	// Clean up created files and directories
	os.RemoveAll("./cmd")
	os.RemoveAll(outputDir)
}

// Test mergeConfigurations method
func TestMergeConfig(t *testing.T){
	logger := initLogger()

	// Create a new multi-loader driver with the test config path
	multiLoader, err := NewMultiLoaderDriver(multiLoaderTestConfigPath, logger, "info", false, false)
	if err != nil {
		t.Fatalf("Failed to create multi-loader driver: %v", err)
	}
	experiment := config.LoaderExperiment{
		Name: "example_1",
		Config: map[string]interface{}{
			"ExperimentDuration": 10,
			"TracePath": "./test_multi_trace/example_1_test",
			"OutputPathPrefix": "./test_output/example_1_test",
		},
	}
	outputConfig := multiLoader.mergeConfigurations("./test_base_loader_config.json", experiment)
	// Check if the configurations are merged
	if outputConfig.TracePath != "./test_multi_trace/example_1_test" {
		t.Errorf("Expected TracePath to be './test_multi_trace/example_1_test', got %v", experiment.Config["TracePath"])
	}
	if outputConfig.OutputPathPrefix != "./test_output/example_1_test" {
		t.Errorf("Expected OutputPathPrefix to be './test_output/example_1_test', got %v", experiment.Config["OutputPathPrefix"])
	}
	if outputConfig.ExperimentDuration != 10 {
		t.Errorf("Expected ExperimentDuration to be 10, got %v", experiment.Config["ExperimentDuration"])
	}
}

