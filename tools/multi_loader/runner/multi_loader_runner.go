package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/vhive-serverless/loader/pkg/common"
	"github.com/vhive-serverless/loader/pkg/config"

	log "github.com/sirupsen/logrus"
)

const (
	TIME_FORMAT = "Jan_02_1504"
)

type MultiLoaderRunner struct {
    MultiLoaderConfig common.MultiLoaderConfiguration
    NodeGroup common.NodeGroup
    DryRunSuccess bool
	Verbosity	string
	IatGeneration	bool
	Generated	bool
	DryRun bool
	Platform string
}

// init multi loader runner
func NewMultiLoaderRunner(configPath string, verbosity string, iatGeneration bool, generated bool) (*MultiLoaderRunner, error) {
    multiLoaderConfig := config.ReadMultiLoaderConfigurationFile(configPath)

	// validate configuration
	common.CheckMultiLoaderConfig(multiLoaderConfig)
	
	// determine platform
	platform := determinePlatform(multiLoaderConfig)

    runner := MultiLoaderRunner{
        MultiLoaderConfig: multiLoaderConfig,
        DryRunSuccess: true,
		Verbosity: verbosity,
		IatGeneration: iatGeneration,
		Generated: generated,
		DryRun: false,
		Platform: platform,
    }
	
	// For knative platform, help to determine and validate nodes in cluster
	if platform == "Knative" || platform == "Knative-RPS" {
		nodeGroup := determineNodes(multiLoaderConfig)
		// add to runner
		runner.NodeGroup = nodeGroup
		common.CheckMultiLoaderPlatformSpecificConfig(multiLoaderConfig, nodeGroup, platform)
	}

	return &runner, nil
}

func determinePlatform(multiLoaderConfig common.MultiLoaderConfiguration) string {
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

func determineNodes(multiLoaderConfig common.MultiLoaderConfiguration) common.NodeGroup {
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

func (d *MultiLoaderRunner) run(){
	// Run global prescript
	common.RunScript(d.MultiLoaderConfig.PreScript)
	// Iterate over studies and run them
	for _, study := range d.MultiLoaderConfig.Studies {
		log.Info("Setting up experiment: ", study.Name)
		// Run pre script
		common.RunScript(study.PreScript)	

		// Unpack study to sparse experiments
		sparseExperiments := d.unpackStudy(study)

		// Run post script
		common.RunScript(study.PostScript)
	}
	// Run global postscript
	common.RunScript(d.MultiLoaderConfig.PostScript)
}

/** 
* As a study can have multiple experiments, this function will unpack the study
* but first by duplicating the study to multiple studies with different values 
* in the config field. Those values will override the base loader config later
*/
func (d *MultiLoaderRunner) unpackStudy(experiment common.LoaderStudy) []common.LoaderStudy {
	log.Info("Unpacking experiment ", experiment.Name)
	var experiments []common.LoaderStudy

	// If user specified a trace directory
	if experiment.TracesDir != "" {
		experiments = d.unpackFromTraceDir(experiment)
	// User Define trace format and values instead of directory
	} else if experiment.TracesFormat != "" && len(experiment.TraceValues) > 0 {
		experiments = d.unpackFromTraceValues(experiment)
	} else {
		// Theres only one experiment in the study
		experiments = d.unpackSingleExperiment(experiment)
	}

	return experiments
}

func (d *MultiLoaderRunner) unpackFromTraceDir(study common.LoaderStudy) []common.LoaderStudy {
	var experiments []common.LoaderStudy
	files, err := os.ReadDir(study.TracesDir)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		newExperiment := d.createNewStudy(study, file.Name())
		experiments = append(experiments, newExperiment)
	}
	return experiments
}

func (d *MultiLoaderRunner) unpackFromTraceValues(study common.LoaderStudy) []common.LoaderStudy {
	var experiments []common.LoaderStudy
	for _, traceValue := range study.TraceValues {
		tracePath := strings.Replace(study.TracesFormat, common.TraceFormatString, fmt.Sprintf("%v", traceValue), -1)
		fileName := path.Base(tracePath)
		newExperiment := d.createNewStudy(study, fileName)
		newExperiment.Config["TracePath"] = tracePath
		newExperiment.Name += "_" + fileName
		experiments = append(experiments, newExperiment)
	}
	return experiments
}

func (d *MultiLoaderRunner) unpackSingleExperiment(study common.LoaderStudy) []common.LoaderStudy {
	var experiments []common.LoaderStudy
	pathDir := ""
	if study.Config["OutputPathPrefix"] != nil {
		pathDir = path.Dir(study.Config["OutputPathPrefix"].(string))
	} else {
		pathDir = study.OutputDir
	}
	study.OutputDir = pathDir
	newExperiment := d.createNewStudy(study, study.Name)
	experiments = append(experiments, newExperiment)
	return experiments
}

func (d *MultiLoaderRunner) createNewStudy(study common.LoaderStudy, fileName string) common.LoaderStudy {
	newStudy, err := common.DeepCopy(study)
	if err != nil {
		log.Fatal(err)
	}

	dryRunAdditionalPath := ""
	if d.DryRun {
		dryRunAdditionalPath = "dry_run"
	}
	newStudy.Config["OutputPathPrefix"] = path.Join(
		study.OutputDir,
		study.Name,
		dryRunAdditionalPath,
		time.Now().Format(TIME_FORMAT)+"_"+fileName,
		fileName,
	)
	d.addCommandFlags(newStudy)
	return newStudy
}

func (d *MultiLoaderRunner) addCommandFlags(study common.LoaderStudy) {
	// Add flags to experiment config
	if study.Verbosity == "" {
		study.Verbosity = d.Verbosity
	}
	if !study.IatGeneration {
		study.IatGeneration = d.IatGeneration
	}
	if !study.Generated {
		study.Generated = d.Generated
	} 
	
}