namespace DomainService.Interfaces;

public static class EntityTypes
{
    public const string Task = "task";
    public const string User = "user";
    public const string UserSettings = "user-settings";
}

public static class TaskEventTypes
{
    public const string Created = "task-created";
    public const string Updated = "task-updated";
    public const string Completed = "task-completed";
    public const string Reopened = "task-reopened";
}

public static class UserEventTypes
{
    public const string Created = "user-created";
    public const string Login = "user-logged-in";
    public const string Logout = "user-logged-out";
    public const string SettingsUpdated = "user-settings-updated";
    public const string SettingsCreated = "user-settings-created";
}

public static class CommandTypes
{
    public const string CompleteTask = "complete-task";
    public const string CreateTask = "create-task";
    public const string UpdateTask = "update-task";
    public const string ReopenTask = "reopen-task";
    public const string LoginUser = "login-user";
    public const string LogoutUser = "logout-user";
    public const string UpdateUserSettings = "update-user-settings";
}
