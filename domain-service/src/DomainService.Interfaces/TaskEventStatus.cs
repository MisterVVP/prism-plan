namespace DomainService.Interfaces;

public readonly struct TaskEventStatus
{
    public static TaskEventStatus NotFound => default;

    public TaskEventStatus(bool exists, bool done)
    {
        Exists = exists;
        Done = done;
    }

    public bool Exists { get; }
    public bool Done { get; }
}
