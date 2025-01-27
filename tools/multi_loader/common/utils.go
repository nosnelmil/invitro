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

func WriteMultiLoaderConfigurationFile(config types.MultiLoaderConfiguration, path string) {
	configByteValue, err := json.Marshal(config)
	if err != nil {
		log.Fatal(err)
	}

	err = os.WriteFile(path, configByteValue, 0644)
	if err != nil {
		log.Fatal(err)
	}
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

func NextProduct(a []int, r int) func() []int {
	p := make([]int, r)
	x := make([]int, len(p))
	return func() []int {
		p := p[:len(x)]
		for i, xi := range x {
			p[i] = a[xi]
		}
		for i := len(x) - 1; i >= 0; i-- {
			x[i]++
			if x[i] < len(a) {
				break
			}
			x[i] = 0
			if i <= 0 {
				x = x[0:0]
				break
			}
		}
		return p
	}
}

func IntArrToString(arr []int) string {
	str := ""
	for _, val := range arr {
		str += string(val)
	}
	return str
}
