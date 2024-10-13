package main

import (
	"encoding/json"
	"os/exec"
	"regexp"

	log "github.com/sirupsen/logrus"
)

type PrometheusSnapshot struct {
	Status 		string 		`json:"status"`
	ErrorType 	string 		`json:"errorType"`
	Error 		string 		`json:"error"` 
	Data 		interface{} `json:"data"`
}

func main() {
	cmd := exec.Command("ssh", "Lenson@hp146.utah.cloudlab.us", "curl -XPOST http://localhost:9090/api/v1/admin/tsdb/snapshot")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Error(string(out))
		log.Fatal(err)
	}
	re := regexp.MustCompile(`\{.*\}`)
	jsonBytes := re.Find(out)
	var prometheusSnapshot PrometheusSnapshot
	err = json.Unmarshal(jsonBytes, &prometheusSnapshot)
	if err != nil {
		log.Fatal(err)
	}
	if prometheusSnapshot.Status != "success" {
		log.Info("Prometheus Snapshot not ready. Retry...")
	}
}