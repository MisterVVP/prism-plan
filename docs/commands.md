# Domain Command Catalog

This catalog lists all domain commands accepted by the system along with their expected payload structures.

```mermaid
flowchart LR
    subgraph Task Commands
        CTC[create-task]
        UTC[update-task]
        CMTC[complete-task]
    end
    subgraph User Commands
        LUC[login-user]
        LOC[logout-user]
        UUS[update-user-settings]
    end
```

| Command | Description | Payload Structure |
|---------|-------------|------------------|
| `create-task` | Create a new task. | `{ "title": string, "notes"?: string, "category"?: string, "order"?: number }` |
| `update-task` | Modify task fields. | `{ "id": string, "title"?: string, "notes"?: string, "category"?: string, "order"?: number, "done"?: boolean }` |
| `complete-task` | Mark a task as completed. | `{ "id": string }` |
| `login-user` | Log a user in, creating the user if they do not exist. | `{ "name": string, "email": string }` |
| `logout-user` | Log a user out. | _No payload_ |
| `update-user-settings` | Change user settings. | `{ "tasksPerCategory"?: number, "showDoneTasks"?: boolean }` |
