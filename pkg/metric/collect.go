/*
 * MIT License
 *
 * Copyright (c) 2023 EASL and the vHive community
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */

package metric

import (
	"encoding/json"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/vhive-serverless/loader/pkg/common"
)

func ScrapeDeploymentScales() []DeploymentScale {
	cmd := exec.Command("python3", "pkg/metric/scrape_scales.py")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Warn("Fail to scrape deployment scales: ", err)
	}

	var results []DeploymentScale
	err = json.Unmarshal(out, &results)
	if err != nil {
		log.Warn("Fail to parse deployment scales: ", string(out[:]), err)
	}

	return results
}

func ScrapeKnStats() KnStats {
	cmd := exec.Command(
		"python3",
		"pkg/metric/scrape_kn.py",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Warn("Fail to scrape Knative: ", err)
	}

	var result KnStats
	err = json.Unmarshal(out, &result)
	if err != nil {
		log.Warn("Fail to parse Knative: ", string(out[:]), err)
	}

	return result
}

func ScrapeClusterUsage() ClusterUsage {
	cmd := exec.Command("python3", "pkg/metric/scrape_infra.py")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Warn("Fail to scrape cluster usage: ", err)
	}

	var result ClusterUsage
	err = json.Unmarshal(out, &result)
	if err != nil {
		log.Warn("Fail to parse cluster usage: ", string(out[:]), err)
	}

	return result
}

func TOPProcessMetrics(nodes []string, outputDir string, reset bool) {
	var wg sync.WaitGroup
	for _, node := range nodes {
		wg.Add(1)
		go func(node string) {
			defer wg.Done()
			// kill all instances of top
			common.RunRemoteCommand(node, "if pgrep top >/dev/null; then killall top; fi")
			if reset {
				// run top in the background
				common.RunRemoteCommand(node, "top -b -d 15 -c -w 512 > top.txt 2>&1 &")
			} else {
				common.CopyRemoteFile(node, "top.txt", path.Join(outputDir, "top_" + node + ".txt"))
			}
		}(strings.TrimSpace(node))
	}

	wg.Wait() 
}

func RetrieveAutoScalerLogs(node string, outputDir string){
	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		log.Fatal(err)
	}
	// Retrieve autoscaler logs
	common.CopyRemoteFile(node, "/var/log/pods/knative-serving_autoscaler-*/autoscaler/*", outputDir)
}

func RetrieveActivatorLogs(node string, outputDir string){
	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		log.Fatal(err)
	}
	// Retrieve activator logs
	common.CopyRemoteFile(node, "/var/log/pods/knative-serving_activator-*/activator/*", outputDir)
}

func RetrievePrometheusSnapshot(node string, outputDir string){
	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		log.Fatal(err)
	}
	// Retrieve prometheus snapshot
	i := 10
	var prometheusSnapshot common.PrometheusSnapshot
	for i > 0{
		cmd := exec.Command("ssh", node, "curl -XPOST http://localhost:9090/api/v1/admin/tsdb/snapshot")
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Warn("Failed to retrieve prometheus snapshot: ", err, "Retrying...")
			i--
			continue
		}
		re := regexp.MustCompile(`\{.*\}`)
		jsonBytes := re.Find(out)
		err = json.Unmarshal(jsonBytes, &prometheusSnapshot)

		if err != nil {
			log.Fatal(err)
		}
		if prometheusSnapshot.Status != "success" {
			if i == 1{
				log.Error("Prometheus Snapshot failed")
			}else{
				log.Info("Prometheus Snapshot not ready. Retrying...")
			}
			i--
			continue
		}
		break
	}
	if prometheusSnapshot.Status != "success" {
		log.Error("Prometheus Snapshot failed")
		return
	}
	// Copy prometheus snapshot to file
	var tempSnapshotDir = "~/tmp/prometheus_snapshot"
	common.RunRemoteCommand(node, "mkdir -p " + tempSnapshotDir)
	common.RunRemoteCommand(node, "kubectl cp -n monitoring " + "prometheus-prometheus-kube-prometheus-prometheus-0:/prometheus/snapshots/ " + 
		"-c prometheus " + tempSnapshotDir)
	common.CopyRemoteFile(node, tempSnapshotDir, path.Dir(outputDir))
	// remove temp directory
	common.RunRemoteCommand(node, "rm -rf " + tempSnapshotDir)
}