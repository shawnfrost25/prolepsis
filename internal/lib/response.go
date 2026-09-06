package lib

type Error struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
	TraceID string            `json:"trace_id,omitempty"`
}
