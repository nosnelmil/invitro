package driver

/*func GenerateColdStartLoads(
	rpsStart int,
	rpsStep int,
	hotFunction tc.Function,
	coldstartCounts []int,
	iatDistribution common.IatDistribution,
	withTracing bool,
	seed int64,
) {

	start := time.Now()
	wg := sync.WaitGroup{}
	collector := mc.NewCollector()
	clusterUsage := mc.ClusterUsage{}
	knStats := mc.KnStats{}
	coldStartMinuteCount := 0*/

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
			coldStartMinuteCount += collector.RecordScalesAndGetColdStartCount()
		}
	}()

	rps := rpsStart
	minute := 0
	tolerance := 0

	sg := generator.NewSpecificationGenerator(seed)

coldstart_generation:
	for {
		iats, _ := sg.GenerateIAT([]int{rps * 60}, iatDistribution)*/
//* One minute per step for matching the trace mode.
/*		tick := -1
		timeout := time.After(1 * time.Minute)
		interval := time.Duration(iats[0][0]) * time.Microsecond
		ticker := time.NewTicker(interval)
		done := make(chan bool, 1)

		//* The following counters are for each RPS step slot.
		var successCountRpsStep int64 = 0
		var failureCountRpsStep int64 = 0

		coldStartTarget := coldstartCounts[minute%len(coldstartCounts)]
		coldStartIndices := generateColdStartTimeIdx(coldStartTarget, len(iats))
		nxtColdStart := 0

		wg.Add(1)
		/** Launch a timer. */
/*go func() {
			defer wg.Done()

			<-timeout
			ticker.Stop()
			done <- true
		}()

		for {
			tick++
			select {
			case <-ticker.C:
				var function tc.Function
				if nxtColdStart < coldStartTarget && tick == coldStartIndices[nxtColdStart] {
					function = collector.GetOneColdStartFunction()
					nxtColdStart++
				} else {
					function = hotFunction
				}

				wg.Add(1)
				go func(rps int, interval int64) {
					defer wg.Done()

					runtimeRequested, memoryRequested := sg.GenerateExecutionSpecs(function)
					success, execRecord := fc.Invoke(function, runtimeRequested, memoryRequested, withTracing)

					if success {
						atomic.AddInt64(&successCountRpsStep, 1)
					} else {
						atomic.AddInt64(&failureCountRpsStep, 1)
					}
					execRecord.Interval = interval
					execRecord.Rps = rps
					collector.ReportExecution(execRecord, clusterUsage, knStats)
				}(rps, interval.Milliseconds()) //* NB: `clusterUsage` needn't be pushed onto the stack as we want the latest.

			case <-done:
				invRecord := mc.MinuteInvocationRecord{
					MinuteIdx:     minute,
					Rps:           rps,
					NumColdStarts: coldStartMinuteCount,
				}
				collector.ReportInvocation(invRecord)
				coldStartMinuteCount = 0

				if CheckOverload(atomic.LoadInt64(&successCountRpsStep), atomic.LoadInt64(&failureCountRpsStep)) {
					tolerance++
					if tolerance < common.OVERFLOAD_TOLERANCE {
						rps -= rpsStep //* Stay in the current RPS for one more time.
						goto next_rps
					} else {
						break coldstart_generation
					}
				} else {
					goto next_rps
				}
			}
		}
	next_rps:
		if rpsStep == 0 { // For a single shot.
			break coldstart_generation
		}
		minute++
		rps += rpsStep
		log.Info("Start next round with RPS=", rps, " after ", time.Since(start))
	}
	log.Info("Finished coldstart generation with ending RPS=", rps)

	forceTimeoutDuration := common.FORCE_TIMEOUT_MINUTE * time.Minute
	if wgWaitWithTimeout(&wg, forceTimeoutDuration) {
		log.Warn("Time out waiting for all invocations to return.")
	} else {
		totalDuration := time.Since(start)
		log.Info("[No timeout] Total invocation + waiting duration: ", totalDuration, "\n")
	}

	defer collector.FinishAndSave(0, 0, rps)
}*/

/*func generateColdStartTimeIdx(target, totalInvocations int) []int {
	indices := []int{}
	if target == 0 {
		return indices
	}

	total := 0
	step := totalInvocations / target
	//* Spread cold starts evenly across all invocations.
	for i := 0; total < target; i += step {
		indices = append(indices, i)
		total++
	}
	return indices
}*/