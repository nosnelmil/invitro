package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/vhive-serverless/loader/pkg/config"
)

const (
	OWNER_READ_WRITE = 0644
	LOADER_RELATIVE_PATH = "../loader.go"
	EXPERIMENT_TEMP_CONFIG_PATH = "current_running_config.json"

)

var (
    multiLoaderConfigPath    = flag.String("multiLoaderConfig", "multi_loader_config.json", "Path to multi loader configuration file")
    verbosity     = flag.String("verbosity", "info", "Logging verbosity - choose from [info, debug, trace]")
    iatGeneration = flag.Bool("iatGeneration", false, "Generate iats only or run invocations as well")
    generated     = flag.Bool("generated", false, "True if iats were already generated")
)

// Initialize logger
func init() {
    flag.Parse()

    log.SetFormatter(&log.TextFormatter{
        TimestampFormat: time.StampMilli,
        FullTimestamp:   true,
    })
    log.SetOutput(os.Stdout)

    switch *verbosity {
    case "debug":
        log.SetLevel(log.DebugLevel)
    case "trace":
        log.SetLevel(log.TraceLevel)
    default:
        log.SetLevel(log.InfoLevel)
    }
}

func main() {
	log.Info("Starting multiloader")
	multiLoaderConfig := config.ReadMultiLoaderConfigurationFile(*multiLoaderConfigPath)

	// Get absolute path of the loader
    loaderPath, err := filepath.Abs(LOADER_RELATIVE_PATH)
    if err != nil {
        log.Fatalf("Failed to get absolute path %s: %v", loaderPath, err)
    }
	// Iterate over experiments and run them
	for _, experiment := range multiLoaderConfig.Experiments {
		// Ask user to confirm the experiment from cli
		log.Info("Do you want to run experiment ", experiment.Name, "? (y/n)")
		var input string
		fmt.Scanln(&input)
		if strings.ToLower(input) == "n" {
			log.Info("Skipping experiment ", experiment.Name)
			continue
		}

		log.Info("Preparing experiment ", experiment.Name)
		// Merge base configs with experiment configs
		experimentConfig := mergeConfigurations(multiLoaderConfig.BaseConfigPath, experiment.Config)
		// Write experiment configs to temp file
		experiementConfigBytes, _ := json.Marshal(experimentConfig);
		err := os.WriteFile(EXPERIMENT_TEMP_CONFIG_PATH, experiementConfigBytes, OWNER_READ_WRITE)
		if err != nil {
			log.Fatal(err)
		}
		defer os.Remove(EXPERIMENT_TEMP_CONFIG_PATH)
		// Call loader.go
		runExperiment(experiment, loaderPath)
	}
	log.Info("All experiments completed")
}

/**
 * Run loader.go with experiment configs
 */
func runExperiment(experiment config.LoaderExperiment,loaderPath string) {
	log.Info("Running experiment ", experiment.Name)
	// Run loader.go with experiment configs
	cmd := exec.Command("go", "run", loaderPath,
		"--config="+EXPERIMENT_TEMP_CONFIG_PATH,
		"--verbosity="+*verbosity,
		fmt.Sprintf("--iatGeneration=%v", *iatGeneration),
		fmt.Sprintf("--generated=%v", *generated))

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	cmd.Start()
	
	go logOutput(stdout, log.WithField("Experiment", experiment.Name).Info)
	go logOutput(stderr, log.WithField("Experiment", experiment.Name).Error)

	cmd.Wait()

	log.Info("Experiment ", experiment.Name, " completed")
}

func logOutput(stdPipe io.ReadCloser, logFunc func(args ...interface{})) {
    scanner := bufio.NewScanner(stdPipe)
    scanner.Split(bufio.ScanLines)
    for scanner.Scan() {
        m := scanner.Text()
		
		if m == "" {continue}
		// extract message from logrus output
		message := strings.Split(m, "msg=")
		if len(message) > 1 {
			m = message[1][1:len(message[1])-1]
		}
        logFunc(m)
	}
}

// Merge base configs with experiment configs
func mergeConfigurations(baseConfigPath string, experimentConfig map[string]interface{}) config.LoaderConfiguration {
	// Read base configuration
	baseConfigByteValue, err := os.ReadFile(baseConfigPath)
	if err != nil {
		log.Fatal(err)
	}
	log.Debug("Experiment configuration ", experimentConfig)
	var mergedConfig config.LoaderConfiguration
	err = json.Unmarshal(baseConfigByteValue, &mergedConfig)
	if err != nil {
		log.Fatal(err)
	}
	
	log.Debug("Base configuration ", mergedConfig)
	experimentConfigBytes, _ := json.Marshal(experimentConfig)
	err = json.Unmarshal(experimentConfigBytes, &mergedConfig);
	if err != nil {
		log.Fatal(err)
	}
	log.Debug("Merged configuration ", mergedConfig)

	return mergedConfig
}