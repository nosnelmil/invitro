package main

import (
	"flag"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/vhive-serverless/loader/pkg/config"
	"github.com/vhive-serverless/loader/pkg/driver"
)

var (
    multiLoaderConfigPath    = flag.String("multiLoaderConfig", "cmd/multi_loader/multi_loader_config.json", "Path to multi loader configuration file")
    verbosity     = flag.String("verbosity", "info", "Logging verbosity - choose from [info, debug, trace]")
    iatGeneration = flag.Bool("iatGeneration", false, "Generate iats only and skip invocations")
	generated     = flag.Bool("generated", false, "If iats were already generated")

	multiLoaderConfig = config.MutliLoaderConfiguration{}
	masterNode, autoscalerNode, activatorNode, loaderNode string
	workerNodes                                         []string

	dryRunSuccess = true

	// Temp flag to run loader on remote node
	syncConfig	 = flag.Bool("syncConfig", false, "sync loader on remote node")
)

func init() {
	flag.Parse()
}

func initLogger() *log.Logger {
	logger := log.New()
	logger.SetFormatter(&log.TextFormatter{
		TimestampFormat: time.StampMilli,
		FullTimestamp:   true,
	})
	logger.SetOutput(os.Stdout)

	switch *verbosity {
	case "debug":
		logger.SetLevel(log.DebugLevel)
	case "trace":
		logger.SetLevel(log.TraceLevel)
	default:
		logger.SetLevel(log.InfoLevel)
	}
	return logger
}

func main() {
	// Initialize logger
	log := initLogger()
	
	log.Info("Starting multiloader")
	// Create multi loader driver
	multiLoaderDriver, err := driver.NewMultiLoaderDriver(*multiLoaderConfigPath, log, *verbosity, *iatGeneration, *generated)
	if err != nil {
		log.Fatalf("Failed to create multi loader driver: %v", err)
	}
	// Dry run
	multiLoaderDriver.RunDryRun()

	if !multiLoaderDriver.DryRunSuccess {
		log.Fatal("Dry run failed. Exiting...")
	}
	// Actual run
	log.Info("Running experiments")
	multiLoaderDriver.RunActual()
	// Finish
	log.Info("All experiments completed")
}