package main

import (
	"bufio"
	"encoding/json"
	"flag"
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
	rw_r__r__ = 0644
	rwxr_xr_x = 0755
	LOADER_PATH = "cmd/loader.go"
	// LOADER_PATH = "cmd/test/test.go"
	EXPERIMENT_TEMP_CONFIG_PATH = "cmd/multi_loader/current_running_config.json"
	NUM_OF_RETRIES = 2
)

var (
    multiLoaderConfigPath    = flag.String("multiLoaderConfig", "cmd/multi_loader/multi_loader_config.json", "Path to multi loader configuration file")
    verbosity     = flag.String("verbosity", "info", "Logging verbosity - choose from [info, debug, trace]")
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
	// Run global prescript
	runScript(multiLoaderConfig.PreScriptPath)
	// Iterate over experiments and run them
	for _, experiment := range multiLoaderConfig.Experiments {
		log.Info("Setting up experiment: ", experiment.Name)
		// Unpack experiment
		subExperiments := unpackExperiment(experiment)
		// Run each experiment
		for _, subExperiment := range subExperiments {
			// Prepare experiment
			prepareExperiment(multiLoaderConfig, subExperiment)		
			// Run pre script
			runScript(subExperiment.PreScriptPath)	
			// Call loader.go
			runExperiment(subExperiment)
			// Run post script
			runScript(subExperiment.PostScriptPath)
			// Perform cleanup
			performCleanup()
		}
		if len(subExperiments) > 1 {
			log.Info("All experiments for ", experiment.Name, " completed")
		}
	}
	// Run global postscript
	runScript(multiLoaderConfig.PostScriptPath)
	// Finish
	log.Info("All experiments completed")
}

func prepareExperiment(multiLoaderConfig config.MutliLoaderConfiguration, subExperiment config.LoaderExperiment) {
	log.Info("Preparing ", subExperiment.Name)
	// Merge base configs with experiment configs
	experimentConfig := mergeConfigurations(multiLoaderConfig.BaseConfigPath, subExperiment)
    
	// Create output directory
	outputDirs := strings.Split(experimentConfig.OutputPathPrefix, "/")
	outputDir := strings.Join(outputDirs[:len(outputDirs)-1], "/")
	err := os.MkdirAll(outputDir, rwxr_xr_x)
	if err != nil {
		log.Fatal(err)
	}
	// Write experiment configs to temp file
	writeExperimentConfigToTempFile(experimentConfig, EXPERIMENT_TEMP_CONFIG_PATH)
}

func unpackExperiment(experiment config.LoaderExperiment) []config.LoaderExperiment {
	log.Info("Unpacking experiment ", experiment.Name)
	subExperiments := []config.LoaderExperiment{}


	if experiment.TracesDir != "" {

		files, err := os.ReadDir(experiment.TracesDir)
		if err != nil {
			log.Fatal(err)
		}

		for _, file := range files {
			var newExperiment config.LoaderExperiment
			// Create a deep copy of experiment
			DeepCopy(experiment, &newExperiment)
			
			// Set new experiment configs based on trace file
			newExperiment.Config["TracePath"] = newExperiment.TracesDir + "/" + file.Name()
			newExperiment.Config["OutputPathPrefix"] = newExperiment.OutputDir + "/" + newExperiment.Name + "/" + file.Name() + "/" + file.Name()
			newExperiment.Name = experiment.Name + "_" + file.Name()
			
			if err != nil {
				log.Fatal(err)
			}
			// Merge base configs with experiment configs
			subExperiments = append(subExperiments, newExperiment)
		}
	} else {
		subExperiments = append(subExperiments, experiment)
	}
	return subExperiments
}

/**
 * Run loader.go with experiment configs
 */
func runExperiment(experiment config.LoaderExperiment) {
	log.Info("Running ", experiment.Name)
	log.Debug("Experiment configuration ", experiment.Config)

	experimentVerbosity := experiment.Verbosity
	if experiment.Verbosity == "" {
		experimentVerbosity = *verbosity
	}

	// Setup logrus logger
	// Create the log file
	experimentOutPutDirArr := strings.Split(experiment.Config["OutputPathPrefix"].(string), "/")
	experimentOutPutDirArr = experimentOutPutDirArr[:len(experimentOutPutDirArr)-1]
	experimentOutPutDir := strings.Join(experimentOutPutDirArr, "/")
	logFilePath := experimentOutPutDir+"/loader.log"

	if _, err := os.Stat(logFilePath); err == nil {
		err := os.Remove(logFilePath)
		if err != nil {
			log.Error(err)
		}
	}
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer logFile.Close()

	for i := 0; i < NUM_OF_RETRIES; i++ {
		if i > 0 {
			log.Info("Retrying experiment ", experiment.Name)
			logFile.WriteString("==================================RETRYING==================================\n")
			experimentVerbosity = "debug"
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

		go logStdOutput(stdout, logFile)
		go logStdError(stderr, logFile)

		err = cmd.Wait()
		if err != nil {
			log.Error(err)
		}
		
		if err != nil {
			log.Error("Experiment failed: ", experiment.Name)
			logFile.WriteString("Experiment failed: " + experiment.Name + "\n")
			continue
		}
		break
	}

	log.Info(experiment.Name, " completed")
}

func performCleanup() {
	log.Info("Runnning Cleanup")
	// Remove temp file
	os.Remove(EXPERIMENT_TEMP_CONFIG_PATH)
	// Run make clean
	cmd := exec.Command("make", "clean")
	cmd.Run()
}


func logStdOutput(stdPipe io.ReadCloser, logFile *os.File) {

    scanner := bufio.NewScanner(stdPipe)
    for scanner.Scan() {
        m := scanner.Text()
		// write to log file
		logFile.WriteString(m + "\n")
		
		// Log key information
		if m == "" {continue}
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
		m = strings.ReplaceAll(m, "\n", "")
		if logType == "debug" {
			log.Debug(m)
		} else if logType == "trace" {
			log.Trace(m)
		} else {
			if strings.Contains(m, "Number of successful invocations:") || strings.Contains(m, "Number of failed invocations:") {
				m = strings.ReplaceAll(m, "\t", "  ",)
				log.Info(m)
			}
		}
	}
}
func logStdError(stdPipe io.ReadCloser, logFile *os.File) {
	scanner := bufio.NewScanner(stdPipe)
	for scanner.Scan() {
		m := scanner.Text()
		// write to log file
		logFile.WriteString(m + "\n")
		
		if m == "" {
			continue
		}
		log.Error(m)
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
		experiment.Config["OutputPathPrefix"] = "data/out/" +experiment.Name 
	}
	experiment.Config["OutputPathPrefix"] = experiment.Config["OutputPathPrefix"].(string) + "_" + time.Now().Format("Jan02_1504")

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
	err := os.WriteFile(fileWritePath, experimentConfigBytes, rw_r__r__)
	if err != nil {
		log.Fatal(err)
	}
}

func runScript(scriptPath string) {
	if scriptPath == "" {
		return
	}
	log.Info("Running script ", scriptPath)
	cmd, err := exec.Command("/bin/sh", scriptPath).Output()
	if err != nil {
		log.Fatal(err)
	}
	log.Info(string(cmd))
}

func DeepCopy(a, b interface{}) {
    byt, _ := json.Marshal(a)
    json.Unmarshal(byt, b)
}
