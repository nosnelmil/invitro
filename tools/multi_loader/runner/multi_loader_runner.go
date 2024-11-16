package runner

import (
	"encoding/json"
	"os"

	"github.com/vhive-serverless/loader/pkg/common"
	"github.com/vhive-serverless/loader/pkg/config"

	log "github.com/sirupsen/logrus"
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
		// Run post script
		common.RunScript(study.PostScript)
	}
	// Run global postscript
	common.RunScript(d.MultiLoaderConfig.PostScript)
}