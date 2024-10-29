package driver

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/vhive-serverless/loader/pkg/common"
	"github.com/vhive-serverless/loader/pkg/config"
	"github.com/vhive-serverless/loader/pkg/metric"

	log "github.com/sirupsen/logrus"
)

const (
	LOADER_PATH = "cmd/loader.go"
	// LOADER_PATH = "cmd/test/test.go"
	EXPERIMENT_TEMP_CONFIG_PATH = "cmd/multi_loader/current_running_config.json"
	INVITRO_BASE_PATH = "~/loader/"
	NUM_OF_RETRIES = 2
	TIME_FORMAT = "Jan_02_1504"
	LOADER_NODE_ADD = "Lenson@pc808.emulab.net"
)

type MultiLoaderDriver struct {
    MultiLoaderConfig config.MutliLoaderConfiguration
    NodeGroup common.NodeGroup
    DryRunSuccess bool
    Logger       *log.Logger
	Verbosity	string
	IatGeneration	bool
	Generated	bool
	DryRun bool
	Platform string
}

// Initialize the Driver with config and logger
func NewMultiLoaderDriver(configPath string, logger *log.Logger, verbosity string, iatGeneration bool, generated bool) (*MultiLoaderDriver, error) {
    multiLoaderConfig := config.ReadMultiLoaderConfigurationFile(configPath)

	// Determine platform
	platform := determinePlatform(multiLoaderConfig)
    
    // Determine nodes (same as in your code)
	var nodeGroup common.NodeGroup
	if platform == common.Knative {
		nodeGroup = determineNodes(multiLoaderConfig)
	}

	// Validate config and nodes
	common.CheckMultiLoaderConfig(multiLoaderConfig, nodeGroup, platform)

    return &MultiLoaderDriver{
        MultiLoaderConfig: multiLoaderConfig,
        NodeGroup: nodeGroup,
        DryRunSuccess: true,
        Logger: logger,
		Verbosity: verbosity,
		IatGeneration: iatGeneration,
		Generated: generated,
		DryRun: false,
		Platform: platform,
    }, nil
}

func determinePlatform(multiLoaderConfig config.MutliLoaderConfiguration) string {
	// Determine platform
	baseConfigByteValue, err := os.ReadFile(multiLoaderConfig.BaseConfigPath)
	if err != nil {
		log.Fatal(err)
	}
	var loaderConfig config.LoaderConfiguration
	// Unmarshal base configuration
	if err = json.Unmarshal(baseConfigByteValue, &loaderConfig); err != nil {
		log.Fatal(err)
	}
	return loaderConfig.Platform
}

func determineNodes(multiLoaderConfig config.MutliLoaderConfiguration) common.NodeGroup {
	var nodeGroup common.NodeGroup
	nodeGroup.MasterNode = multiLoaderConfig.MasterNode
	nodeGroup.AutoScalerNode = multiLoaderConfig.AutoScalerNode
	nodeGroup.ActivatorNode = multiLoaderConfig.ActivatorNode
	nodeGroup.LoaderNode = multiLoaderConfig.LoaderNode
	nodeGroup.WorkerNodes = multiLoaderConfig.WorkerNodes

	if len(nodeGroup.WorkerNodes) == 0 {
		nodeGroup.WorkerNodes = common.DetermineWorkerNodes()
	}
	if nodeGroup.MasterNode == "" {
		nodeGroup.MasterNode = common.DetermineMasterNode()
	}
	if nodeGroup.LoaderNode  == "" {
		nodeGroup.LoaderNode = common.DetermineLoaderNode()
	}
	if nodeGroup.AutoScalerNode == "" {
		nodeGroup.AutoScalerNode = common.DetermineOtherNodes("autoscaler")
	}
	if nodeGroup.ActivatorNode == "" {
		nodeGroup.ActivatorNode = common.DetermineOtherNodes("activator")
	}
	return nodeGroup
}

func (d *MultiLoaderDriver) RunDryRun() {
    d.Logger.Info("Running dry run")
    d.DryRun = true
    d.runMultiLoader()
}

func (d *MultiLoaderDriver) RunActual() {
    d.Logger.Info("Running actual experiments")
    d.DryRun = false
    d.runMultiLoader()
}

func (d *MultiLoaderDriver) runMultiLoader(){
	// Run global prescript
	common.RunScript(d.MultiLoaderConfig.PreScript)
	// Iterate over experiments and run them
	for _, experiment := range d.MultiLoaderConfig.Experiments {
		d.Logger.Info("Setting up experiment: ", experiment.Name)
		// Unpack experiment
		subExperiments := d.unpackExperiment(experiment)
		// Run pre script
		common.RunScript(experiment.PreScript)	
		// Run each experiment
		for _, subExperiment := range subExperiments {
			if d.DryRun{
				log.Info("Dry Running: ", subExperiment.Name)
			}
			
			// Prepare experiment
			d.prepareExperiment(subExperiment)		
			// Call loader.go
			shouldContinue := d.runExperiment(subExperiment)
			// Collect logs
			if !d.DryRun {
				d.collateMetrics(subExperiment)
			}
			// Perform cleanup
			d.performCleanup()
			// Check if should continue
			if !shouldContinue {
				log.Info("Experiment failed: ", subExperiment.Name, ". Skipping remaining experiments in study...")
				break
			}
		}
		// Run post script
		common.RunScript(experiment.PostScript)
		if len(subExperiments) > 1 && !d.DryRun{
			log.Info("All experiments for ", experiment.Name, " completed")
		}
	}
	// Run global postscript
	common.RunScript(d.MultiLoaderConfig.PostScript)
}

// The role of this function is just to create partial loader configs 
// and the values will override values in the base loader config later
func (d *MultiLoaderDriver) unpackExperiment(experiment config.LoaderExperiment) []config.LoaderExperiment {
	log.Info("Unpacking experiment ", experiment.Name)
	var subExperiments []config.LoaderExperiment

	// If user specified a trace directory
	if experiment.TracesDir != "" {
		subExperiments = d.unpackFromTraceDir(experiment)
	// User Define trace format and values instead of directory
	} else if experiment.TracesFormat != "" && len(experiment.TraceValues) > 0 {
		subExperiments = d.unpackFromTraceValues(experiment)
	} else {
		// Theres only one experiment in the study
		subExperiments = d.unpackSingleExperiment(experiment)
	}

	return subExperiments
}

func (d *MultiLoaderDriver) unpackFromTraceDir(experiment config.LoaderExperiment) []config.LoaderExperiment {
	var subExperiments []config.LoaderExperiment
	files, err := os.ReadDir(experiment.TracesDir)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		newExperiment := d.createNewExperiment(experiment, file.Name())
		subExperiments = append(subExperiments, newExperiment)
	}
	return subExperiments
}

func (d *MultiLoaderDriver) unpackFromTraceValues(experiment config.LoaderExperiment) []config.LoaderExperiment {
	var subExperiments []config.LoaderExperiment
	for _, traceValue := range experiment.TraceValues {
		tracePath := strings.Replace(experiment.TracesFormat, common.TraceFormatString, fmt.Sprintf("%v", traceValue), -1)
		fileName := path.Base(tracePath)
		newExperiment := d.createNewExperiment(experiment, fileName)
		newExperiment.Config["TracePath"] = tracePath
		newExperiment.Name += "_" + fileName
		subExperiments = append(subExperiments, newExperiment)
	}
	return subExperiments
}

func (d *MultiLoaderDriver) unpackSingleExperiment(experiment config.LoaderExperiment) []config.LoaderExperiment {
	var subExperiments []config.LoaderExperiment
	pathDir := path.Dir(experiment.Config["OutputPathPrefix"].(string))
	experiment.OutputDir = pathDir
	newExperiment := d.createNewExperiment(experiment, experiment.Name)
	subExperiments = append(subExperiments, newExperiment)
	return subExperiments
}

func (d *MultiLoaderDriver) createNewExperiment(experiment config.LoaderExperiment, fileName string) config.LoaderExperiment {
	newExperiment, err := common.DeepCopy(experiment)
	if err != nil {
		log.Fatal(err)
	}

	dryRunAdditionalPath := ""
	if d.DryRun {
		dryRunAdditionalPath = "dry_run"
	}
	newExperiment.Config["OutputPathPrefix"] = path.Join(
		experiment.OutputDir,
		experiment.Name,
		dryRunAdditionalPath,
		time.Now().Format(TIME_FORMAT)+"_"+fileName,
		fileName,
	)
	d.addCommandFlags(newExperiment)
	return newExperiment
}

func (d *MultiLoaderDriver) addCommandFlags(experiment config.LoaderExperiment) {
	// Add flags to experiment config
	if experiment.Verbosity == "" {
		experiment.Verbosity = d.Verbosity
	}
	if !experiment.IatGeneration {
		experiment.IatGeneration = d.IatGeneration
	}
	if !experiment.Generated {
		experiment.Generated = d.Generated
	} 
	
}

func (d *MultiLoaderDriver) prepareExperiment(subExperiment config.LoaderExperiment) {
	log.Info("Preparing ", subExperiment.Name)
	// Merge base configs with experiment configs
	experimentConfig := d.mergeConfigurations(d.MultiLoaderConfig.BaseConfigPath, subExperiment)
    
	// Create output directory
	outputDir := path.Dir(experimentConfig.OutputPathPrefix)

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatal(err)
	}
	// Write experiment configs to temp file
	d.writeExperimentConfigToTempFile(experimentConfig, EXPERIMENT_TEMP_CONFIG_PATH)

	if d.shouldCollectMetric(common.TOP) {
		// Reset TOP
		d.topProcessMetrics(outputDir, true)
	}
}

func (d *MultiLoaderDriver) topProcessMetrics(experimentPath string, reset bool) {
	nodes := []string{d.NodeGroup.MasterNode, d.NodeGroup.LoaderNode}
	if d.NodeGroup.AutoScalerNode != d.NodeGroup.MasterNode {
		nodes = append(nodes, d.NodeGroup.AutoScalerNode)
	}
	if d.NodeGroup.ActivatorNode != d.NodeGroup.MasterNode {
		nodes = append(nodes, d.NodeGroup.ActivatorNode)
	}
	nodes = append(nodes, d.NodeGroup.WorkerNodes...)

	metric.TOPProcessMetrics(nodes, experimentPath, reset)
}

/**
* Merge base configs with partial loader configs
*/
func (d *MultiLoaderDriver) mergeConfigurations(baseConfigPath string, experiment config.LoaderExperiment) config.LoaderConfiguration {
	// Read base configuration
	baseConfigByteValue, err := os.ReadFile(baseConfigPath)
	if err != nil {
		log.Fatal(err)
	}
	log.Debug("Experiment configuration ", experiment.Config)
	
	var mergedConfig config.LoaderConfiguration
	// Unmarshal base configuration
	if err = json.Unmarshal(baseConfigByteValue, &mergedConfig); err != nil {
		log.Fatal(err)
	}

	log.Debug("Base configuration ", mergedConfig)
	
	// merge experiment config onto base config
	experimentConfigBytes, _ := json.Marshal(experiment.Config)
	if err = json.Unmarshal(experimentConfigBytes, &mergedConfig); err != nil {
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
func (d *MultiLoaderDriver) runExperiment(experiment config.LoaderExperiment) bool {
	log.Info("Running ", experiment.Name)
	log.Debug("Experiment configuration ", experiment.Config)

	// Create the log file
	logFilePath := path.Join(path.Dir(experiment.Config["OutputPathPrefix"].(string)), "loader.log")
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer logFile.Close()

	for i := 0; i < NUM_OF_RETRIES; i++ {
		// Run loader.go with experiment configs
		if err := d.executeLoaderCommand(experiment, logFile); err != nil {
			log.Error(err)
			log.Error("Experiment failed: ", experiment.Name)
			logFile.WriteString("Experiment failed: " + experiment.Name + ". Error: " + err.Error() + "\n")
			if i == 0 && !d.DryRun {
				log.Info("Retrying experiment ", experiment.Name)
				logFile.WriteString("==================================RETRYING==================================\n")
				experiment.Verbosity = "debug"
			} else{
				// Experiment failed set dry run flag to false
				d.DryRunSuccess = false
				log.Error("Check log file for more information: ", logFilePath)
				// should not continue with experiment
				return false
			}
			continue
		}else{
			break
		}
	}
	log.Info("Completed ", experiment.Name)
	return true
}

func (d *MultiLoaderDriver) executeLoaderCommand(experiment config.LoaderExperiment, logFile *os.File) error {
	cmd := exec.Command("go", "run", LOADER_PATH,
		"--config="+EXPERIMENT_TEMP_CONFIG_PATH,
		"--verbosity="+experiment.Verbosity,
		"--iatGeneration="+strconv.FormatBool(experiment.IatGeneration),
		"--generated="+strconv.FormatBool(experiment.Generated),
		"--dryRun="+strconv.FormatBool(d.DryRun))

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	
	if err := cmd.Start(); err != nil {
		return err
	}
	
	go d.logLoaderStdOutput(stdout, logFile)
	go d.logLoaderStdError(stderr, logFile)
	
	return cmd.Wait()
}

func (d *MultiLoaderDriver) logLoaderStdOutput(stdPipe io.ReadCloser, logFile *os.File) {

	scanner := bufio.NewScanner(stdPipe)
	for scanner.Scan() {
		m := scanner.Text()
		// write to log file
		logFile.WriteString(m + "\n")
		
		// Log key information
		if m == "" {
			continue
		}
		logType := common.ParseLogType(m)
		message := common.ParseLogMessage(m)
		
		switch logType {
		case "debug":
			log.Debug(message)
		case "trace":
			log.Trace(message)
		default:
			if strings.Contains(message, "Number of successful invocations:") || strings.Contains(message, "Number of failed invocations:") {
				log.Info(strings.ReplaceAll(message, "\\t", " ",))
			}
		}
	}
}

func (d *MultiLoaderDriver) logLoaderStdError(stdPipe io.ReadCloser, logFile *os.File) {
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

func (d *MultiLoaderDriver) collateMetrics(experimentConfig config.LoaderExperiment) {
	// Check if should collect metrics
	if len(d.MultiLoaderConfig.Metrics) == 0 {
		return
	}
	// collate Metrics
	log.Info("Collating Metrics")
	experimentDir := path.Dir(experimentConfig.Config["OutputPathPrefix"].(string))
	
	if(d.shouldCollectMetric(common.TOP)) {
		// Collect top Metrics
		topDir := path.Join(experimentDir, "top")
		if err := os.MkdirAll(topDir, 0755); err != nil {
			log.Fatal(err)
		}
		d.topProcessMetrics(topDir, false)
	}
	
	if(d.shouldCollectMetric(common.AutoScaler)) {
		// Retrieve auto scaler logs
		autoScalerLogDir := path.Join(experimentDir, "autoscaler")
		metric.RetrieveAutoScalerLogs(d.NodeGroup.AutoScalerNode, autoScalerLogDir)
	}

	if(d.shouldCollectMetric(common.Activator)) {
		// Retrieve activator logs
		activatorLogDir := path.Join(experimentDir, "activator")
		metric.RetrieveActivatorLogs(d.NodeGroup.ActivatorNode, activatorLogDir)
	}
	
	if(d.shouldCollectMetric(common.Prometheus)) {
		// Retrieve prometheus snapshot
		prometheusSnapshotDir := path.Join(experimentDir, "prometheus_snapshot")
		metric.RetrievePrometheusSnapshot(d.NodeGroup.MasterNode, prometheusSnapshotDir)
	}
}

func (d *MultiLoaderDriver) performCleanup() {
	log.Info("Runnning Cleanup")
	// Run make clean
	if err := exec.Command("make", "clean").Run(); err != nil {
		log.Error(err)
	}
	log.Info("Cleanup completed")
	// Remove temp file
	os.Remove(EXPERIMENT_TEMP_CONFIG_PATH)
}

func (d *MultiLoaderDriver) writeExperimentConfigToTempFile(experimentConfig config.LoaderConfiguration, fileWritePath string) {
	experimentConfigBytes, _ := json.Marshal(experimentConfig)
	err := os.WriteFile(fileWritePath, experimentConfigBytes, 0644)
	if err != nil {
		log.Fatal(err)
	}
}

// Helper functions
func (d *MultiLoaderDriver) shouldCollectMetric(targetMetrics string) bool {
	// Only collect for Knative
	if (d.Platform != common.Knative) {
		return false
	}
	for _, metric := range d.MultiLoaderConfig.Metrics {
		if metric == targetMetrics {
			return true
		}
	}
	return false
}

// TEMPORARY FUNCTIONS
func SyncLoaderConfig() {
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

