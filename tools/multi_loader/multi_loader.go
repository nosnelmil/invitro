package main

import (
	"flag"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
)

var (
    verbosity     = flag.String("verbosity", "info", "Logging verbosity - choose from [info, debug, trace]")
)

func init() {
	flag.Parse()
	initLogger()
}

func initLogger() {
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
	// Run multi loader
}
