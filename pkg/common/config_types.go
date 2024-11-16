package common

type MultiLoaderConfiguration struct {
	Studies 		[]LoaderStudy 			`json:"Studies"`
	BaseConfigPath  string             		`json:"BaseConfigPath"`
	// Optional
	PreScript 		string 					`json:"PreScript"`
	PostScript	 	string 					`json:"PostScript"`
	MasterNode 		string 					`json:"MasterNode"`
	AutoScalerNode 	string 					`json:"AutoScalerNode"`
	ActivatorNode 	string 					`json:"ActivatorNode"`
	LoaderNode 		string 					`json:"LoaderNode"`
	WorkerNodes 	[]string 				`json:"WorkerNodes"`
	Metrics         []string				`json:"Metrics"`
}

type LoaderStudy struct {
	Name    		string           		`json:"Name"`
	Config  		map[string]interface{}	`json:"Config"`
	// A combination of format and values or just dir should be specified
	TracesDir 		string 					`json:"TracesDir"`

	TracesFormat 	string 					`json:"TracesFormat"`
	TraceValues 	[]interface{}			`json:"TraceValues"`

	// Optional
	OutputDir 		string 					`json:"OutputDir"`
	Verbosity		string 			   		`json:"Verbosity"`
	IatGeneration 	bool 					`json:"IatGeneration"`
	Generated 		bool 					`json:"Generated"`
	PreScript	 	string 					`json:"PreScript"`
	PostScript	 	string 					`json:"PostScript"`
}