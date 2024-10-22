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
	"github.com/vhive-serverless/loader/pkg/common"
	"github.com/vhive-serverless/loader/pkg/config"
	"github.com/vhive-serverless/loader/pkg/metric"
)

const (
	rw_r__r__ = 0644
	rwxr_xr_x = 0755
	LOADER_PATH = "cmd/loader.go"
	// LOADER_PATH = "cmd/test/test.go"
	EXPERIMENT_TEMP_CONFIG_PATH = "cmd/multi_loader/current_running_config.json"
	INVITRO_BASE_PATH = "~/loader/"
	NUM_OF_RETRIES = 2
	TIME_FORMAT = "Jan_02_1504"
	LOADER_NODE_ADD = "Lenson@pc717.emulab.net"
)

var (
    multiLoaderConfigPath    = flag.String("multiLoaderConfig", "cmd/multi_loader/multi_loader_config.json", "Path to multi loader configuration file")
    verbosity     = flag.String("verbosity", "info", "Logging verbosity - choose from [info, debug, trace]")
    iatGeneration = flag.Bool("iatGeneration", false, "Generate iats only and skip invocations")
	generated     = flag.Bool("generated", false, "If iats were already generated")

	multiLoaderConfig = config.MutliLoaderConfiguration{}
	masterNode = ""
	autoscalerNode = ""
	activatorNode = ""
	loaderNode = ""
	workerNodes = []string{}
	dryRunSuccess = true
	// Temp flag to run loader on remote node
	syncConfig	 = flag.Bool("syncConfig", false, "sync loader on remote node")
)

func init() {
	flag.Parse()
	// Initialize logger
	initLogger()
	// Initialise global variables
	multiLoaderConfig = config.ReadMultiLoaderConfigurationFile(*multiLoaderConfigPath)
	// Determine nodes addresses
	if !*syncConfig {
		determineNodes()
	}
}


func initLogger() {
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

func determineNodes() {
	masterNode = multiLoaderConfig.MasterNode
	autoscalerNode = multiLoaderConfig.AutoScalerNode
	activatorNode = multiLoaderConfig.ActivatorNode
	loaderNode = multiLoaderConfig.LoaderNode
	workerNodes = multiLoaderConfig.WorkerNodes

	if len(workerNodes) == 0 {
		workerNodes = common.DetermineWorkerNodes()
	}
	if masterNode == "" {
		masterNode = common.DetermineMasterNode()
	}
	if loaderNode == "" {
		loaderNode = common.DetermineLoaderNode()
	}
	if autoscalerNode == "" {
		autoscalerNode = common.DetermineOtherNodes("autoscaler")
	}
	if activatorNode == "" {
		activatorNode = common.DetermineOtherNodes("activator")
	}
}


func main() {
	log.Info("Starting multiloader")
	// Sync config. Used for testing
	if *syncConfig {
		syncLoaderConfig()
		return
	}
	// Check multi loader configuration
	common.CheckMultiLoaderConfig(multiLoaderConfig, masterNode, autoscalerNode, activatorNode, loaderNode, workerNodes)
	// Dry run
	log.Info("Starting dry run")
	runMultiLoader(true)
	if !dryRunSuccess {
		log.Fatal("Dry run failed. Exiting...")
	}
	log.Info("Dry run completed")
	log.Info("Running experiments")
	runMultiLoader(false)
	// Finish
	log.Info("All experiments completed")
	
}


func runMultiLoader(dryRun bool){
	// Run global prescript
	common.RunScript(multiLoaderConfig.PreScript)
	// Iterate over experiments and run them
	for _, experiment := range multiLoaderConfig.Experiments {
		log.Info("Setting up experiment: ", experiment.Name)
		// Unpack experiment
		subExperiments := unpackExperiment(experiment, dryRun)
		// Run pre script
		common.RunScript(experiment.PreScript)	
		// Run each experiment
		for _, subExperiment := range subExperiments {
			if dryRun{
				log.Info("Dry Running: ", subExperiment.Name)
			}
			// Prepare experiment
			prepareExperiment(subExperiment)		
			// Call loader.go
			shouldContinue := runExperiment(subExperiment, dryRun)
			// Collect logs
			if !dryRun {
				collateLogs(subExperiment)
			}
			// Perform cleanup
			performCleanup()
			if !shouldContinue {
				log.Info("Experiment failed: ", subExperiment.Name, ". Skipping remaining experiments in study...")
				break
			}
		}
		// Run post script
		common.RunScript(experiment.PostScript)
		if len(subExperiments) > 1 && !dryRun{
			log.Info("All experiments for ", experiment.Name, " completed")
		}
	}
	// Run global postscript
	common.RunScript(multiLoaderConfig.PostScript)
}

// The role of this function is just to create partial loader configs 
// and the values will override values in the base loader config later
func unpackExperiment(experiment config.LoaderExperiment, dryRun bool) []config.LoaderExperiment {
	log.Info("Unpacking experiment ", experiment.Name)
	subExperiments := []config.LoaderExperiment{}

	// If user specified a trace directory
	if experiment.TracesDir != "" {
		files, err := os.ReadDir(experiment.TracesDir)
		if err != nil {
			log.Fatal(err)
		}
		// Create an experiment config for each trace file
		for _, file := range files {
			// deep copy experiment
			newExperiment, err := common.DeepCopy(experiment)
			if err != nil {
				log.Fatal(err)
			}
			
			// Set new experiment configs based on trace file
			newExperiment.Config["TracePath"] = path.Join(experiment.TracesDir, file.Name())
			dryRunAdditionalPath := ""
			if dryRun {
				dryRunAdditionalPath = "dry_run"
			}
			newExperiment.Config["OutputPathPrefix"] = path.Join(
				experiment.OutputDir, 
				experiment.Name, 
				dryRunAdditionalPath,
				time.Now().Format(TIME_FORMAT) + "_" + file.Name(), 
				file.Name())
			newExperiment.Name = file.Name()
			addCommandFlags(newExperiment)
			// Merge base configs with experiment configs
			subExperiments = append(subExperiments, newExperiment)
		}
	// User Define trace format and values instead of directory
	} else if experiment.TracesFormat != "" && len(experiment.TraceValues) > 0 {
		// Create a experiemnt config for each trace value
		for _, traceValue := range experiment.TraceValues {
			// deep copy experiment
			newExperiment, err := common.DeepCopy(experiment)
			if err != nil {
				log.Fatal(err)
			}
			
			tracePath := strings.Replace(experiment.TracesFormat, common.TraceFormatString, fmt.Sprintf("%v", traceValue), -1)
			fileName := path.Base(tracePath)
			// Set new experiment configs based on trace value
			newExperiment.Config["TracePath"] = tracePath
			dryRunAdditionalPath := ""
			if dryRun {
				dryRunAdditionalPath = "dry_run"
			}
			newExperiment.Config["OutputPathPrefix"] = path.Join(
				newExperiment.OutputDir, 
				newExperiment.Name, 
				dryRunAdditionalPath,
				time.Now().Format(TIME_FORMAT) + "_" + fileName, 
				fileName)
			newExperiment.Name = newExperiment.Name + "_" + fileName
			addCommandFlags(newExperiment)
			// Merge base configs with experiment configs
			subExperiments = append(subExperiments, newExperiment)
		}
	} else {
		// Theres only one experiment in the study
		// check if experiment config has the OutputPathPrefix field
		pathDir := path.Dir(experiment.Config["OutputPathPrefix"].(string))
		dryRunAdditionalPath := ""
		if dryRun {
			dryRunAdditionalPath = "dry_run"
		}
		experiment.Config["OutputPathPrefix"] = path.Join(
			pathDir,
			experiment.Name,
			dryRunAdditionalPath,
			time.Now().Format(TIME_FORMAT) + "_" + experiment.Name,
		) 
		addCommandFlags(experiment)
		subExperiments = append(subExperiments, experiment)
	}

	return subExperiments
}

func addCommandFlags(experiment config.LoaderExperiment) {
	// Add flags to experiment config
	if experiment.Verbosity == "" {
		experiment.Verbosity = *verbosity
	}
	if !experiment.IatGeneration {
		experiment.IatGeneration = *iatGeneration
	}
	if !experiment.Generated {
		experiment.Generated = *generated
	} 
	
}

func prepareExperiment(subExperiment config.LoaderExperiment) {
	log.Info("Preparing ", subExperiment.Name)
	// Merge base configs with experiment configs
	experimentConfig := mergeConfigurations(multiLoaderConfig.BaseConfigPath, subExperiment)
    
	// Create output directory
	outputDir := path.Dir(experimentConfig.OutputPathPrefix)
	err := os.MkdirAll(outputDir, rwxr_xr_x)
	if err != nil {
		log.Fatal(err)
	}
	// Write experiment configs to temp file
	writeExperimentConfigToTempFile(experimentConfig, EXPERIMENT_TEMP_CONFIG_PATH)

	// Reset TOP
	topProcessMetrics(outputDir, true)
}

func topProcessMetrics(experimentPath string, reset bool) {
	nodes := []string{masterNode, loaderNode}
	if autoscalerNode != masterNode {
		nodes = append(nodes, autoscalerNode)
	}
	if activatorNode != masterNode {
		nodes = append(nodes, activatorNode)
	}
	nodes = append(nodes, workerNodes...)

	metric.TOPProcessMetrics(nodes, experimentPath, reset)
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
 * @param experiment config.LoaderExperiment
 * @param dryRun bool
 * @return should experiment continue
 */
func runExperiment(experiment config.LoaderExperiment, dryRun bool) bool {
	log.Info("Running ", experiment.Name)
	log.Debug("Experiment configuration ", experiment.Config)

	// Create the log file
	experimentOutPutDir := path.Dir(experiment.Config["OutputPathPrefix"].(string))
	logFilePath := path.Join(experimentOutPutDir, "loader.log")

	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer logFile.Close()

	for i := 0; i < NUM_OF_RETRIES; i++ {
		// Run loader.go with experiment configs
		cmd := exec.Command("go", "run", LOADER_PATH,
			"--config=" + EXPERIMENT_TEMP_CONFIG_PATH,
			"--verbosity=" + experiment.Verbosity,
			"--iatGeneration=" + strconv.FormatBool(experiment.IatGeneration),
			"--generated=" + strconv.FormatBool(experiment.Generated),
			"--dryRun=" + strconv.FormatBool(dryRun))
	
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()

		cmd.Start()

		go logLoaderStdOutput(stdout, logFile)
		go logLoaderStdError(stderr, logFile)

		err = cmd.Wait()
		if err != nil {
			log.Error(err)
			log.Error("Experiment failed: ", experiment.Name)
			logFile.WriteString("Experiment failed: " + experiment.Name + ". Error: " + err.Error() + "\n")
			if i == 0 && !dryRun {
				log.Info("Retrying experiment ", experiment.Name)
				logFile.WriteString("==================================RETRYING==================================\n")
				experiment.Verbosity = "debug"
			} else{
				// Experiment failed set dry run flag to false
				dryRunSuccess = false
				log.Error("Check log file for more information: ", logFilePath)
				// should not continue with experiment
				return false
			}
			continue
		}
		break
	}
	log.Info("Completed ", experiment.Name)
	return true
}

func collateLogs(experimentConfig config.LoaderExperiment) {
	// collate logs
	log.Info("Collating logs")
	experimentDir := path.Dir(experimentConfig.Config["OutputPathPrefix"].(string))
	
	// Create log directories
	topDir := path.Join(experimentDir, "top")
	autoScalerLogDir := path.Join(experimentDir, "autoscaler")
	activatorLogDir := path.Join(experimentDir, "activator")
	prometheusSnapshotDir := path.Join(experimentDir, "prometheus_snapshot")
	
	err := os.MkdirAll(topDir, rwxr_xr_x)
	if err != nil {
		log.Fatal(err)
	}
	
	// Collect top logs
	topProcessMetrics(topDir, false)
	// Retrieve auto scaler logs
	metric.RetrieveAutoScalerLogs(autoscalerNode, autoScalerLogDir)
	// Retrieve activator logs
	metric.RetrieveActivatorLogs(activatorNode, activatorLogDir)
	// Retrieve prometheus snapshot
	metric.RetrievePrometheusSnapshot(masterNode, prometheusSnapshotDir)
}

func performCleanup() {
	log.Info("Runnning Cleanup")
	// Run make clean
	cmd := exec.Command("make", "clean")
	_, err := cmd.CombinedOutput()
	if err != nil {
		log.Error(err)
	}
	log.Info("Cleanup completed")
	// Remove temp file
	os.Remove(EXPERIMENT_TEMP_CONFIG_PATH)
}

func logLoaderStdOutput(stdPipe io.ReadCloser, logFile *os.File) {

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
		m = strings.ReplaceAll(m, "\\n", "")
		if logType == "debug" {
			log.Debug(m)
		} else if logType == "trace" {
			log.Trace(m)
		} else {
			if strings.Contains(m, "Number of successful invocations:") || strings.Contains(m, "Number of failed invocations:") {
				m = strings.ReplaceAll(m, "\\t", " ",)
				log.Info(m)
			}
		}
	}
}

func logLoaderStdError(stdPipe io.ReadCloser, logFile *os.File) {
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

// TEMPORARY FUNCTIONS
func syncLoaderConfig() {
	log.Info("Running loader on remote node")
	// Sync multi-loader configurations
	log.Info("Syncing multi-loader configurations")
	syncToRemoteFile(LOADER_NODE_ADD, "./cmd/multi_loader/", INVITRO_BASE_PATH + "cmd/multi_loader")
	// Sync trace files
	log.Info("Syncing trace files")
	syncToRemoteFile(LOADER_NODE_ADD, "./data/traces/", INVITRO_BASE_PATH + "data/traces")
	// Sync scripts
	log.Info("Syncing scripts")
	syncToRemoteFile(LOADER_NODE_ADD, "./scripts/", INVITRO_BASE_PATH + "scripts")

	log.Info("Done syncing")
}

func syncToRemoteFile(remoteNode string, src string, dest string) {
	cmd := exec.Command("rsync", "-a", src, remoteNode + ":" + dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err:= cmd.Run(); err != nil {
		log.Fatal(err)
	}
}