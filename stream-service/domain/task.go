package domain

type Task struct {
	ID       string `json:"id"`
	Title    string `json:"title,omitempty"`
	Notes    string `json:"notes,omitempty"`
	Category string `json:"category,omitempty"`
	Order    int    `json:"order,omitempty"`
	Done     *bool  `json:"done,omitempty"`
}

type UserSettings struct {
	TasksPerCategory *int  `json:"tasksPerCategory,omitempty"`
	ShowDoneTasks    *bool `json:"showDoneTasks,omitempty"`
}

func boolPtr(v bool) *bool {
	return &v
}
