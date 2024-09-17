package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/vhive-serverless/loader/pkg/config"
)

const (
	OWNER_READ_WRITE = 0644
	LOADER_PATH = "cmd/loader.go"
	EXPERIMENT_TEMP_CONFIG_PATH = "cmd/multi_loader/current_running_config.json"

)

var (
    multiLoaderConfigPath    = flag.String("multiLoaderConfig", "cmd/multi_loader/multi_loader_config.json", "Path to multi loader configuration file")
    verbosity     = flag.String("verbosity", "info", "Logging verbosity - choose from [info, debug, trace]")
	prompt = flag.Bool("prompt", true, "Prompt user to confirm experiment")
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

	// Iterate over experiments and run them
	for _, experiment := range multiLoaderConfig.Experiments {
		// Ask user to confirm the experiment from cli
		if *prompt && !promptUser(experiment.Name){
			continue
		}
		log.Info("Preparing experiment ", experiment.Name)
		// Merge base configs with experiment configs
		experimentConfig := mergeConfigurations(multiLoaderConfig.BaseConfigPath, experiment)
		// Write experiment configs to temp file
		writeExperimentConfigToTempFile(experimentConfig, EXPERIMENT_TEMP_CONFIG_PATH)
		// Call loader.go
		runExperiment(experiment)
		// Remove temp file
		os.Remove(EXPERIMENT_TEMP_CONFIG_PATH)
	}
	log.Info("All experiments completed")
}

/**
 * Run loader.go with experiment configs
 */
func runExperiment(experiment config.LoaderExperiment) {
	log.Info("Running experiment ", experiment.Name)
	log.Debug("Experiment configuration ", experiment.Config)

	experimentVerbosity := experiment.Verbosity
	if experiment.Verbosity == "" {
		experimentVerbosity = *verbosity
	}
	// Run loader.go with experiment configs
	cmd := exec.Command("go", "run", LOADER_PATH,
		"--config=" + EXPERIMENT_TEMP_CONFIG_PATH,
		"--verbosity=" + experimentVerbosity,
		"--iatGeneration=" + strconv.FormatBool(experiment.IatGeneration),
		"--generated=" + strconv.FormatBool(experiment.Generated))

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	cmd.Start()
	
	go logStdOutput(stdout, experiment.Name)
	go logStdError(stderr, experiment.Name)

	cmd.Wait()

	log.Info("Experiment ", experiment.Name, " completed")
}

func logStdOutput(stdPipe io.ReadCloser, experimentName string) {
    scanner := bufio.NewScanner(stdPipe)
    scanner.Split(bufio.ScanLines)
    for scanner.Scan() {
        m := scanner.Text()
		
		if m == "" {continue}
		// extract message from logrus output
		logTypeArr := strings.Split(m, "level=")
		var logType string
		if len(logTypeArr) > 1 {
			logType = strings.Split(logTypeArr[1], " ")[0]
		} else {
			logType = "info"
		}
		message := strings.Split(m, "msg=")
		if len(message) > 1 {
			m = message[1][1:len(message[1])-1]
		}
		if logType == "debug" {
			log.WithField("Experiment", experimentName).Debug(m)
		} else if logType == "trace" {
			log.WithField("Experiment", experimentName).Trace(m)
		} else {
			log.WithField("Experiment", experimentName).Info(m)
		}
	}
}
func logStdError(stdPipe io.ReadCloser, experimentName string) {
    scanner := bufio.NewScanner(stdPipe)
	// scanner.Scan()
    // scanner.Split(bufio.ScanLines)
    for scanner.Scan() {
        m := scanner.Text()
		
		if m == "" {continue}
		log.WithField("Experiment", experimentName).Error(m)
	}
}

// Merge base configs with experiment configs
func mergeConfigurations(baseConfigPath string, experiment config.LoaderExperiment) config.LoaderConfiguration {
	// Read base configuration
	baseConfigByteValue, err := os.ReadFile(baseConfigPath)
	if err != nil {
		log.Fatal(err)
	}
	log.Debug("Experiment configuration ", experiment.Config)
	var mergedConfig config.LoaderConfiguration
	err = json.Unmarshal(baseConfigByteValue, &mergedConfig)
	if err != nil {
		log.Fatal(err)
	}
	
	log.Debug("Base configuration ", mergedConfig)
	// check if experiment config has a field: OutputPathPrefix
	if _, ok := experiment.Config["OutputPathPrefix"]; !ok {
		experiment.Config["OutputPathPrefix"] = "data/out/" +experiment.Name + "_" + time.Now().Format("Jan02_1504")
	}
	experimentConfigBytes, _ := json.Marshal(experiment.Config)
	err = json.Unmarshal(experimentConfigBytes, &mergedConfig);
	if err != nil {
		log.Fatal(err)
	}
	log.Debug("Merged configuration ", mergedConfig)

	return mergedConfig
}

func writeExperimentConfigToTempFile(experimentConfig config.LoaderConfiguration, fileWritePath string) {
	experimentConfigBytes, _ := json.Marshal(experimentConfig)
	err := os.WriteFile(fileWritePath, experimentConfigBytes, OWNER_READ_WRITE)
	if err != nil {
		log.Fatal(err)
	}
}

func promptUser(experimentName string) bool {
	log.Info("Do you want to run experiment ", experimentName, "? (y/n)")
	var input string
	fmt.Scanln(&input)
	if strings.ToLower(input) == "n" {
		log.Info("Skipping experiment ", experimentName)
		return false
	}
	return true
}