package common

import (
	"bytes"
	"net"
	"os"
	"os/exec"
	"path"
	"slices"
	"strings"

	log "github.com/sirupsen/logrus"
)

func CheckNode(node string) {
	if !IsValidIP(node) {
		log.Fatal("Invalid IP address for node ", node)
	}
	cmd := exec.Command("ssh", "-oStrictHostKeyChecking=no", "-p", "22", node, "exit")
	// -oStrictHostKeyChecking=no -p 22
	out, err := cmd.CombinedOutput()
	if bytes.Contains(out, []byte("Permission denied")) || err != nil {
		log.Error(string(out))
		log.Fatal("Failed to connect to node ", node)
	}
}

func CheckPath(path string) {
	if(path) == "" { return }
	_, err := os.Stat(path)
	if err != nil {
		log.Fatal(err)
	}
}

func IsValidIP(ip string) bool {
    parsedIP := net.ParseIP(ip)
    return parsedIP != nil
}

// Check general multi-loader configuration that applies to all platforms
func CheckMultiLoaderConfig(multiLoaderConfig MutliLoaderConfiguration) {
	log.Info("Checking multi-loader configuration")
	// Check if all paths are valid
	CheckPath(multiLoaderConfig.BaseConfigPath)
	// Check each experiments
	if len(multiLoaderConfig.Experiments) == 0 {
		log.Fatal("No experiments found in configuration file")
	}
	for _, experiment := range multiLoaderConfig.Experiments {
		// Check trace directory
		// if configs does not have TracePath or OutputPathPreix, either TracesDir or (TracesFormat and TraceValues) should be defined along with OutputDir
		if experiment.TracesDir == "" && (experiment.TracesFormat == "" || len(experiment.TraceValues) == 0) {
			if _, ok := experiment.Config["TracePath"]; !ok {
				log.Fatal("Missing one of TracesDir, TracesFormat & TraceValues, Config.TracePath in multi_loader_config ", experiment.Name)
			}
		}
		if experiment.TracesFormat != ""{
			// check if trace format contains TRACE_FORMAT_STRING
			if !strings.Contains(experiment.TracesFormat, TraceFormatString) {
				log.Fatal("Invalid TracesFormat in multi_loader_config ", experiment.Name, ". Missing ", TraceFormatString, " in format")
			}
		}
		if experiment.OutputDir == "" {
			if _, ok := experiment.Config["OutputPathPrefix"]; !ok {
				log.Warn("Missing one of OutputDir or Config.OutputPathPrefix in multi_loader_config ", experiment.Name)
				// set default output directory
				experiment.OutputDir = path.Join("data", "out", experiment.Name)
				log.Warn("Setting default output directory to ", experiment.OutputDir)
			}
		}
	}
	log.Info("All experiments configs are valid")
}

// check platform specific multi-loader configuration
func CheckMultiLoaderPlatformSpecificConfig(multiLoaderConfig MutliLoaderConfiguration, nodeGroup NodeGroup, platform string) {
	log.Info("Checking platform specific multi-loader configuration")
	// For knative platform, there is an additional check for nodes
	if platform == "Knative" || platform == "Knative-RPS" {
		// Check if metrics are valid
		for _, metric := range multiLoaderConfig.Metrics {
			CheckCollectableMetrics(metric)
		}
		// Check nodes
		CheckNode(nodeGroup.MasterNode)
		CheckNode(nodeGroup.AutoScalerNode)
		CheckNode(nodeGroup.ActivatorNode)
		CheckNode(nodeGroup.LoaderNode)
		for _, node := range nodeGroup.WorkerNodes {
			CheckNode(node)
		}
		log.Info("Nodes are reachable")
	}
	log.Info("All platform specific configs are valid")
}

func CheckCollectableMetrics(metrics string) {
	if !slices.Contains(ValidCollectableMetrics, metrics) {
		log.Fatal("Invalid metrics ", metrics)
	}
}