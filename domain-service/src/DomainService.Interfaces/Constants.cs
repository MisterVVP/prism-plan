namespace DomainService.Interfaces;

public static class EntityTypes
{
    public const string Task = "task";
}

public static class TaskEventTypes
{
    public const string Created = "task-created";
    public const string Updated = "task-updated";
    public const string Completed = "task-completed";
}
