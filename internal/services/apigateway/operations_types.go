package apigateway

type AdminOperationsAlert struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Source   string `json:"source"`
	Summary  string `json:"summary"`
	Detail   string `json:"detail,omitempty"`
}

type AdminOperationsStatus struct {
	Healthy     bool                   `json:"healthy"`
	GeneratedAt string                 `json:"generated_at"`
	Alerts      []AdminOperationsAlert `json:"alerts"`
}
