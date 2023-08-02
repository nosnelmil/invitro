package metric

type ScaleRegistry struct {
	scaleGauge map[string]int
}

func (r *ScaleRegistry) Init(records []ScaleRecord) {
	r.scaleGauge = map[string]int{}
	for _, record := range records {
		r.scaleGauge[record.Deployment] = record.ActualScale
	}
}

//! Since all functions are deployed once, we assume no duplications.
func (r *ScaleRegistry) UpdateAndGetColdStartCount(records []ScaleRecord) int {
	coldStarts := 0
	for _, record := range records {
		prevScale := r.scaleGauge[record.Deployment]
		currScale := record.ActualScale

		//* Check if it's scaling from 0.
		if prevScale == 0 && currScale > 0 {
			coldStarts++
		}
		//* Update registry.
		r.scaleGauge[record.Deployment] = currScale
	}
	return coldStarts
}

func (r *ScaleRegistry) GetOneColdFunctionName() string {
	for f, scale := range r.scaleGauge {
		if scale == 0 {
			return f
		}
	}
	return "None"
}