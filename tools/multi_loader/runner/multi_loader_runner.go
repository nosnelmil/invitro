package runner

import (
	"encoding/json"
	"os"

	"github.com/vhive-serverless/loader/pkg/common"
	"github.com/vhive-serverless/loader/pkg/config"

	log "github.com/sirupsen/logrus"
)

type MultiLoaderRunner struct {
    MultiLoaderConfig common.MutliLoaderConfiguration
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

func determinePlatform(multiLoaderConfig common.MutliLoaderConfiguration) string {
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

func determineNodes(multiLoaderConfig common.MutliLoaderConfiguration) common.NodeGroup {
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
	// Iterate over experiments and run them
	for _, experiment := range d.MultiLoaderConfig.Experiments {
		log.Info("Setting up experiment: ", experiment.Name)
		// Run pre script
		common.RunScript(experiment.PreScript)	
		// Run post script
		common.RunScript(experiment.PostScript)
	}
	// Run global postscript
	common.RunScript(d.MultiLoaderConfig.PostScript)
}