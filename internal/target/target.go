package target

type Target struct {
	Name    string       `json:"name"`
	Options LocalOptions `json:"options"`
}

type LocalOptions struct {
	Location string `json:"location"`
}
