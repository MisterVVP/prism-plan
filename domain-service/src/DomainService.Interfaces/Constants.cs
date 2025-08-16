namespace DomainService.Interfaces;

public static class EntityTypes
{
    public const string Task = "task";
    public const string User = "user";
}

public static class TaskEventTypes
{
    public const string Created = "task-created";
    public const string Updated = "task-updated";
    public const string Completed = "task-completed";
}

public static class CommandTypes
{
    public const string CompleteTask = "complete-task";
    public const string CreateTask = "create-task";
    public const string UpdateTask = "update-task";
    public const string LoginUser = "login-user";
    public const string LogoutUser = "logout-user";
}
