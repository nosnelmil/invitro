package common

type PrometheusSnapshot struct {
	Status 		string 		`json:"status"`
	ErrorType 	string 		`json:"errorType"`
	Error 		string 		`json:"error"` 
	Data 		interface{} `json:"data"`
}
