package main

type Control struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type Metric struct {
	Timestamp float64                `json:"timestamp"`
	Metric    string                 `json:"metric"`
	Value     float64                `json:"value"`
	Tags      map[string]interface{} `json:"tags"`
}

type Message struct {
	Control Control  `json:"control"`
	Metrics []Metric `json:"metrics"`
}
