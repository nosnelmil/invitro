package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/vhive-serverless/loader/pkg/config"
)

type PrometheusSnapshot struct {
	Status 		string 		`json:"status"`
	ErrorType 	string 		`json:"errorType"`
	Error 		string 		`json:"error"` 
	Data 		interface{} `json:"data"`
}

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
	runRemote	 = flag.Bool("runRemote", true, "Run loader on remote node")
	masterNode = ""
	autoscalerNode = ""
	activatorNode = ""
	loaderNode = ""
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
	masterNode = multiLoaderConfig.MasterNode
	autoscalerNode = multiLoaderConfig.AutoScalerNode
	activatorNode = multiLoaderConfig.ActivatorNode
	loaderNode = multiLoaderConfig.LoaderNode
	// Check config
	// checkMultiLoaderConfig(multiLoaderConfig)
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
			// Collect logs
			collateLogs(subExperiment)
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
	if *runRemote {
		runRemoteCommand(loaderNode, "mkdir -p " + "invitro/" + outputDir)
	}
	err := os.MkdirAll(outputDir, rwxr_xr_x)
	if err != nil {
		log.Fatal(err)
	}
	// Write experiment configs to temp file
	writeExperimentConfigToTempFile(experimentConfig, EXPERIMENT_TEMP_CONFIG_PATH)
	if *runRemote {
		log.Info("Copying experiment config to loader node")
		cmd := exec.Command("scp", EXPERIMENT_TEMP_CONFIG_PATH, loaderNode + ":invitro/" + path.Dir(EXPERIMENT_TEMP_CONFIG_PATH))
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Error(string(out))
			log.Fatal(err)
		}
	}
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

	logFilePath := path.Join(experimentOutPutDir, "loader.log")

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
		var cmd *exec.Cmd
		if *runRemote {
			cmd = exec.Command("ssh", loaderNode, "pushd ~/invitro && source /etc/profile && go run", LOADER_PATH,
				"--config=" + EXPERIMENT_TEMP_CONFIG_PATH,
				"--verbosity=" + experimentVerbosity,
				"--iatGeneration=" + strconv.FormatBool(experiment.IatGeneration),
				"--generated=" + strconv.FormatBool(experiment.Generated))
		}else{
			// Run loader.go with experiment configs
			cmd = exec.Command("go", "run", LOADER_PATH,
				"--config=" + EXPERIMENT_TEMP_CONFIG_PATH,
				"--verbosity=" + experimentVerbosity,
				"--iatGeneration=" + strconv.FormatBool(experiment.IatGeneration),
				"--generated=" + strconv.FormatBool(experiment.Generated))
	
		}
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()

		cmd.Start()

		go logLoaderStdOutput(stdout, logFile)
		go logLoaderStdError(stderr, logFile)

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

	log.Info("Completed ", experiment.Name)
}

func collateLogs(experimentConfig config.LoaderExperiment) {
	// collate logs
	log.Info("Collating logs")
	experimentDir := path.Dir(experimentConfig.Config["OutputPathPrefix"].(string))
	// Create autoscaler log directory
	autoScalerLogDir := path.Join(experimentDir, "autoscaler")
	activatorLogDir := path.Join(experimentDir, "activator")
	prometheusSnapshotDir := path.Join(experimentDir, "prometheus_snapshot")

	err := os.MkdirAll(autoScalerLogDir, rwxr_xr_x)
	if err != nil {
		log.Fatal(err)
	}
	err = os.MkdirAll(activatorLogDir, rwxr_xr_x)
	if err != nil {
		log.Fatal(err)
	}
	err = os.MkdirAll(prometheusSnapshotDir, rwxr_xr_x)
	if err != nil {
		log.Fatal(err)
	}
	// Copy results
	copyRemoteFile(loaderNode, "~/invitro/" + experimentConfig.Config["OutputPathPrefix"].(string)+"*", experimentDir)
	// Retrieve auto scaler logs
	copyRemoteFile(autoscalerNode, "/var/log/pods/knative-serving_autoscaler-*/autoscaler/*", autoScalerLogDir)
	// Retrieve activator logs
	copyRemoteFile(activatorNode, "/var/log/pods/knative-serving_activator-*/activator/*", activatorLogDir)
	// Retrieve prometheus snapshot
	i := 10
	var prometheusSnapshot PrometheusSnapshot
	for i > 0{
		cmd := exec.Command("ssh", masterNode, "curl -XPOST http://localhost:9090/api/v1/admin/tsdb/snapshot")
		out, err := cmd.CombinedOutput()
		if err != nil {
			continue
		}
		re := regexp.MustCompile(`\{.*\}`)
		jsonBytes := re.Find(out)
		err = json.Unmarshal(jsonBytes, &prometheusSnapshot)

		if err != nil {
			log.Fatal(err)
		}
		if prometheusSnapshot.Status != "success" {
			if i == 1{
				log.Error("Prometheus Snapshot failed")
				break
			}else{
				log.Info("Prometheus Snapshot not ready. Retry...")
			}
			i--
			continue
		}
		break
	}
	if prometheusSnapshot.Status != "success" {
		log.Error("Prometheus Snapshot failed")
		return
	}
	// Copy prometheus snapshot to file
	var tempSnapshotDir = "~/tmp/prometheus_snapshot"
	runRemoteCommand(masterNode, "mkdir -p " + tempSnapshotDir)
	runRemoteCommand(masterNode, "kubectl cp -n monitoring " + "prometheus-prometheus-kube-prometheus-prometheus-0:/prometheus/snapshots/ " + 
		"-c prometheus " + tempSnapshotDir)
	copyRemoteFile(masterNode, tempSnapshotDir, prometheusSnapshotDir)
}

func runRemoteCommand(ip string, command string){
	cmd := exec.Command("ssh", "-oStrictHostKeyChecking=no", "-p 22", ip, command)
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	cmd.Start()
	go func () {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			m := scanner.Text()
			log.Info(m)
		}
	}()
	go func () {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			m := scanner.Text()
			log.Error(m)
		}
	}()
	cmd.Wait()
}

func performCleanup() {
	log.Info("Runnning Cleanup")
	if *runRemote {
		runRemoteCommand(loaderNode ,"rm invitro/" + EXPERIMENT_TEMP_CONFIG_PATH)
		runRemoteCommand(loaderNode ,"pushd invitro && source /etc/profile && make clean")
	}else{
		// Run make clean
		cmd := exec.Command("make", "clean")
		cmd.Run()
	}
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

func copyRemoteFile(remoteNode, src string, dest string) {
	cmd := exec.Command("scp", "-rp", remoteNode + ":" + src, dest)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Error(string(out))
		log.Fatal(err)
	}
}

// Surface level check
func checkMultiLoaderConfig(multiLoaderConfig config.MutliLoaderConfiguration) {
	log.Info("Checking multi-loader configuration")
	// check if nodes if runRemote is true
	checkNode(multiLoaderConfig.MasterNode)
	checkNode(multiLoaderConfig.AutoScalerNode)
	checkNode(multiLoaderConfig.ActivatorNode)
	if *runRemote {
		checkNode(multiLoaderConfig.LoaderNode)
	}
	log.Info("Nodes are reachable")
	// Check if all paths are valid
	checkPath(multiLoaderConfig.BaseConfigPath)
	checkPath(multiLoaderConfig.PreScriptPath)
	checkPath(multiLoaderConfig.PostScriptPath)
	log.Info("Global scripts are valid")
	// Check each experiments
	if len(multiLoaderConfig.Experiments) == 0 {
		log.Fatal("No experiments found in configuration file")
	}
	for _, experiment := range multiLoaderConfig.Experiments {
		// Check script paths
		checkPath(experiment.PreScriptPath)
		checkPath(experiment.PostScriptPath)
		// Check trace directory
		// if configs does not have TracePath or OutputPathPreix, either TracesDir or (TracesFormat and TraceValues) should be defined along with OutputDir
		if experiment.TracesDir == "" && (experiment.TracesFormat == "" || len(experiment.TraceValues) == 0) {
			if _, ok := experiment.Config["TracePath"]; !ok {
				log.Fatal("Missing TracePath in experiment ", experiment.Name)
			}
		}
		if _, ok := experiment.Config["OutputPathPrefix"]; !ok {
			log.Fatal("Missing OutputPathPrefix in experiment ", experiment.Name)
		}
	}
	log.Info("All experiments configs are valid")
}

func checkNode(node string) {
	if node == "" {
		log.Fatal("Missing Master/AutoScaler/Activator/Loader node in configuration file")
	}
	cmd := exec.Command("ssh -oStrictHostKeyChecking=no -p 22", node, "exit")
	// -oStrictHostKeyChecking=no -p 22
	out, err := cmd.CombinedOutput()
	if bytes.Contains(out, []byte("Permission denied")) || err != nil {
		log.Error(string(out))
		log.Fatal("Cant connect to node ", node)
	}
}

func checkPath(path string) {
	if(path) == "" { return }
	_, err := os.Stat(path)
	if err != nil {
		log.Fatal(err)
	}
}

