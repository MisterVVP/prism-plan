package domain

type Task struct {
        ID       string `json:"id"`
        Title    string `json:"title,omitempty"`
        Notes    string `json:"notes,omitempty"`
        Category string `json:"category,omitempty"`
        Order    int    `json:"order"`
        Done     *bool  `json:"done,omitempty"`
}

type UserSettings struct {
	TasksPerCategory *int  `json:"tasksPerCategory,omitempty"`
	ShowDoneTasks    *bool `json:"showDoneTasks,omitempty"`
}
