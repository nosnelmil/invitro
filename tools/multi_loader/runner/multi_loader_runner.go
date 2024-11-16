package runner

import (
	"github.com/vhive-serverless/loader/pkg/common"
	"github.com/vhive-serverless/loader/pkg/config"
)

type MultiLoaderRunner struct {
    MultiLoaderConfig config.MutliLoaderConfiguration
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

    return &MultiLoaderRunner{
        MultiLoaderConfig: multiLoaderConfig,
        DryRunSuccess: true,
		Verbosity: verbosity,
		IatGeneration: iatGeneration,
		Generated: generated,
		DryRun: false,
    }, nil
}