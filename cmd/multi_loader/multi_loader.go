package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
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
	TIME_FORMAT = "Jan_02_1504"
	TRACE_FORMAT_STRING = "{}"
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
		// Run pre script
		runScript(experiment.PreScriptPath)	
		// Run each experiment
		for _, subExperiment := range subExperiments {
			// Prepare experiment
			prepareExperiment(multiLoaderConfig, subExperiment)		
			// Call loader.go
			runExperiment(subExperiment)
			// Perform cleanup
			performCleanup()
		}
		// Run post script
		runScript(experiment.PostScriptPath)
		if len(subExperiments) > 1 {
			log.Info("All experiments for ", experiment.Name, " completed")
		}
	}
	// Run global postscript
	runScript(multiLoaderConfig.PostScriptPath)
	// Finish
	log.Info("All experiments completed")
}

// The role of this function is just to create partial loader configs 
// and the values will override values in the base loader config later
func unpackExperiment(experiment config.LoaderExperiment) []config.LoaderExperiment {
	log.Info("Unpacking experiment ", experiment.Name)
	subExperiments := []config.LoaderExperiment{}

	log.Info("TEST", experiment.TracesDir, experiment.TracesFormat, experiment.TraceValues)
	// If user specified a trace directory
	if experiment.TracesDir != "" {
		files, err := os.ReadDir(experiment.TracesDir)
		if err != nil {
			log.Fatal(err)
		}
		// Create a experiemnt config for each trace file

		// TODO: loop through arr of value defined in config
		for _, file := range files {
			var newExperiment config.LoaderExperiment
			// deep copy experiment
			DeepCopy(experiment, &newExperiment)
			
			// Set new experiment configs based on trace file
			newExperiment.Config["TracePath"] = path.Join(newExperiment.TracesDir, file.Name())
			newExperiment.Config["OutputPathPrefix"] = path.Join(
				newExperiment.OutputDir, 
				newExperiment.Name, 
				time.Now().Format(TIME_FORMAT) + "_" + file.Name(), 
				file.Name())
			newExperiment.Name = file.Name()
			
			// Merge base configs with experiment configs
			subExperiments = append(subExperiments, newExperiment)
		}
	// User Define trace format and values instead of directory
	} else if experiment.TracesFormat != "" && len(experiment.TraceValues) > 0 {
		// Create a experiemnt config for each trace value
		for _, traceValue := range experiment.TraceValues {
			var newExperiment config.LoaderExperiment
			// deep copy experiment
			DeepCopy(experiment, &newExperiment)
			
			tracePath := strings.Replace(experiment.TracesFormat, TRACE_FORMAT_STRING, fmt.Sprintf("%v", traceValue), -1)
			log.Info("TRACE PATH", tracePath)
			fileName := path.Base(tracePath)
			// Set new experiment configs based on trace value
			newExperiment.Config["TracePath"] = tracePath
			newExperiment.Config["OutputPathPrefix"] = path.Join(
				newExperiment.OutputDir, 
				newExperiment.Name, 
				time.Now().Format(TIME_FORMAT) + "_" + fileName, 
				fileName)
			newExperiment.Name = newExperiment.Name + "_" + fileName
			
			// Merge base configs with experiment configs
			subExperiments = append(subExperiments, newExperiment)
		}
	} else {
		// Theres only one experiment in the study
		// check if experiment config has the OutputPathPrefix field
		if _, ok := experiment.Config["OutputPathPrefix"]; !ok {
			experiment.Config["OutputPathPrefix"] = "data/out/" + time.Now().Format(TIME_FORMAT) + "_" + experiment.Name 
		}
		subExperiments = append(subExperiments, experiment)
	}
	return subExperiments
}
func prepareExperiment(multiLoaderConfig config.MutliLoaderConfiguration, subExperiment config.LoaderExperiment) {
	log.Info("Preparing ", subExperiment.Name)
	// Merge base configs with experiment configs
	experimentConfig := mergeConfigurations(multiLoaderConfig.BaseConfigPath, subExperiment)
    
	// Create output directory
	outputDirs := strings.Split(experimentConfig.OutputPathPrefix, "/")
	outputDir := path.Join(outputDirs[:len(outputDirs)-1]...)
	err := os.MkdirAll(outputDir, rwxr_xr_x)
	if err != nil {
		log.Fatal(err)
	}
	// Write experiment configs to temp file
	writeExperimentConfigToTempFile(experimentConfig, EXPERIMENT_TEMP_CONFIG_PATH)
}

/**
* Merge base configs with partial loader configs
*/
func mergeConfigurations(baseConfigPath string, experiment config.LoaderExperiment) config.LoaderConfiguration {
	// Read base configuration
	baseConfigByteValue, err := os.ReadFile(baseConfigPath)
	if err != nil {
		log.Fatal(err)
	}
	log.Debug("Experiment configuration ", experiment.Config)
	var mergedConfig config.LoaderConfiguration
	// Unmarshal base configuration
	err = json.Unmarshal(baseConfigByteValue, &mergedConfig)
	if err != nil {
		log.Fatal(err)
	}
	log.Debug("Base configuration ", mergedConfig)
	
	// merge experiment config onto base config
	experimentConfigBytes, _ := json.Marshal(experiment.Config)
	err = json.Unmarshal(experimentConfigBytes, &mergedConfig);
	if err != nil {
		log.Fatal(err)
	}
	log.Debug("Merged configuration ", mergedConfig)

	return mergedConfig
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

