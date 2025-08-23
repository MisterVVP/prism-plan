package domain

// Settings represents user configurable options.
type Settings struct {
        TasksPerCategory int  `json:"tasksPerCategory"`
        ShowDoneTasks    bool `json:"displayDoneTasks"`
}
