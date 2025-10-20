package domain

// Task represents a single board item in the read model.
type Task struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Notes    string `json:"notes,omitempty"`
	Category string `json:"category"`
	Order    int    `json:"order"`
	Done     bool   `json:"done,omitempty"`
}
