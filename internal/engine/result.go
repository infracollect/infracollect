package engine

type Result struct {
	ID   string            `json:"id"`
	Data any               `json:"data"`
	Meta map[string]string `json:"meta,omitempty"`
}
