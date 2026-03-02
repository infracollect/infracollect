package engine

type Result struct {
	ID   string            `json:"id,omitempty"`
	Data any               `json:"data"`
	Meta map[string]string `json:"meta,omitempty"`
}
