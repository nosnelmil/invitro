package common

import (
	"encoding/json"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/vhive-serverless/loader/pkg/config"
	"github.com/vhive-serverless/loader/tools/multi_loader/types"
)

func ReadMultiLoaderConfigurationFile(path string) types.MultiLoaderConfiguration {
	byteValue, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}

	var config types.MultiLoaderConfiguration
	err = json.Unmarshal(byteValue, &config)
	if err != nil {
		log.Fatal(err)
	}

	return config
}

func DeterminePlatformFromConfig(multiLoaderConfig types.MultiLoaderConfiguration) string {
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
