package driver

import (
	"github.com/eth-easl/loader/pkg/common"
)

func GenerateStressLoads(
	rpsStart int,
	rpsEnd int,
	rpsStep int,
	stressSlotInMinutes int,
	functions []common.Function,
	iatDistribution common.IatDistribution,
	withTracing bool,
	seed int64,
) {
	/*sg := generator.NewSpecificationGenerator(seed)

	start := time.Now()
	wg := sync.WaitGroup{}
	collector := mc.NewCollector()
	clusterUsage := mc.ClusterUsage{}
	knStats := mc.KnStats{}
	coldStartSlotCount := 0
	runtimeRequested, memoryRequested := sg.GenerateExecutionSpecs(functions[0])

	/** Launch a scraper that updates the cluster usage every 15s (max. interval). */
	/*scrape_infra := time.NewTicker(time.Second * 15)
	go func() {
		for {
			<-scrape_infra.C
			clusterUsage = mc.ScrapeClusterUsage()
		}
	}()*/

	/** Launch a scraper that updates Knative states every 15s (max. interval). */
	/*scrape_kn := time.NewTicker(time.Second * 15)
	go func() {
		for {
			<-scrape_kn.C
			knStats = mc.ScrapeKnStats()
		}
	}()*/

	/** Launch a scraper for getting cold-start count. */
	/*scrape_scales := time.NewTicker(time.Second * 60)
		go func() {
			for {
				<-scrape_scales.C
				coldStartSlotCount += collector.RecordScalesAndGetColdStartCount()
			}
		}()

		rps := rpsStart
		tolerance := 0

	rps_gen:
		for {
			var invocations []int
			for i := 0; i < stressSlotInMinutes; i++ {
				invocations = append(invocations, rps*60)
			}

			iats, _ := sg.GenerateIAT(invocations, iatDistribution)

			timeout := time.After(time.Minute * time.Duration(stressSlotInMinutes))
			interval := time.Duration(iats[0][0]) * time.Microsecond
			ticker := time.NewTicker(interval)
			done := make(chan bool, 2)
			tick := 0

			//* The following counters are for each RPS step slot.
			var successCountRpsStep int64 = 0
			var failureCountRpsStep int64 = 0
			var numFuncInvokedThisSlot int64 = 0

			wg.Add(1)*/
	/** Launch a timer. */
	/*go func() {
		defer wg.Done()

		<-timeout
		ticker.Stop()
		done <- true
	}()*/

	/*for {
			select {
			case <-ticker.C:
				//* Invoke functions using round robin.
				function := functions[tick%len(functions)]

				wg.Add(1)
				go func(_tick int, _rps int, _interval int64) {
					defer wg.Done()

					atomic.AddInt64(&numFuncInvokedThisSlot, 1)
					success, execRecord := fc.Invoke(function, runtimeRequested, memoryRequested, withTracing)

					if success {
						atomic.AddInt64(&successCountRpsStep, 1)
					} else {
						atomic.AddInt64(&failureCountRpsStep, 1)
					}

					totalInvocationsThisSlot := _rps * 60
					if float64(_tick)/float64(totalInvocationsThisSlot) > common.RPS_WARMUP_FRACTION {

						execRecord.Interval = _interval
						execRecord.Rps = _rps
						collector.ReportExecution(execRecord, clusterUsage, knStats)
					}
				}(tick, rps, interval.Milliseconds()) //* NB: `clusterUsage` needn't be pushed onto the stack as we want the latest.

				if numFuncInvokedThisSlot >= int64(rps*60) ||
					tick >= rps*60 {
					//* Finished before timeout.
					log.Info("Finish target invocation early at RPS=", rps)
					done <- true
				} else {
					interval = time.Duration(iats[0][tick]) * time.Microsecond
					ticker = time.NewTicker(interval)
				}
				tick++

			case <-done:
				invRecord := mc.MinuteInvocationRecord{
					Rps:             rps,
					Duration:        int64(60),
					NumFuncTargeted: rps * 60,
					NumFuncInvoked:  int(numFuncInvokedThisSlot),
				}
				//* Export metrics for all phases.
				collector.ReportInvocation(invRecord)
				goto next_rps
			}
		}

	next_rps:
		if rpsEnd < 0 {
			if CheckOverload(atomic.LoadInt64(&successCountRpsStep), atomic.LoadInt64(&failureCountRpsStep)) {*/
	/** Ending RPS NOT specified -> run until it breaks. */
	/*tolerance++
			if tolerance < common.OVERFLOAD_TOLERANCE {
				rps -= rpsStep //* Second chance: try the current RPS one more time.
			} else {
				break rps_gen
			}
		}
	} else if rps >= rpsEnd || rpsStep == 0 {*/
	/** Ending RPS specified. */
	/*break rps_gen
		}

		if rps < 100 {
			rps += util.MinOf(common.MAX_RPS_STARTUP_STEP, rpsStep)
		} else {
			rps += rpsStep
		}
		log.Info("Start next round with RPS=", rps, " after ", time.Since(start))
	}
	log.Info("Finished stress load generation with ending RPS=", rps)

	forceTimeoutDuration := common.FORCE_TIMEOUT_MINUTE * time.Minute
	if wgWaitWithTimeout(&wg, forceTimeoutDuration) {
		log.Warn("Time out waiting for all invocations to return.")
	} else {
		totalDuration := time.Since(start)
		log.Info("[No timeout] Total invocation + waiting duration: ", totalDuration, "\n")
	}

	defer collector.FinishAndSave(0, 0, rps*60)*/
}