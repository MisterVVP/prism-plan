using DomainService.Domain;
using DomainService.Interfaces;
using System.Text.Json;
using Xunit;

public class TaskStateBuilderTests
{
    [Fact]
    public void From_orders_events_by_timestamp()
    {
        var created = new Event("e1", "t1", EntityTypes.Task, TaskEventTypes.Created,
            JsonDocument.Parse("{\"title\":\"t\"}").RootElement, 0, "u1", "ik1");
        var completed = new Event("e2", "t1", EntityTypes.Task, TaskEventTypes.Completed,
            null, 2, "u1", "ik2");
        var reopened = new Event("e3", "t1", EntityTypes.Task, TaskEventTypes.Updated,
            JsonDocument.Parse("{\"done\":false}").RootElement, 3, "u1", "ik3");
        // shuffled order
        var state = TaskStateBuilder.From(new IEvent[] { completed, created, reopened });
        Assert.False(state.Done);
    }

    [Fact]
    public void Update_does_not_change_done_when_missing()
    {
        var created = new Event("e1", "t1", EntityTypes.Task, TaskEventTypes.Created,
            JsonDocument.Parse("{\"title\":\"t\"}").RootElement, 0, "u1", "ik1");
        var updated = new Event("e2", "t1", EntityTypes.Task, TaskEventTypes.Updated,
            JsonDocument.Parse("{\"title\":\"new\"}").RootElement, 1, "u1", "ik2");
        var final = TaskStateBuilder.From(new IEvent[] { created, updated });
        Assert.False(final.Done);
    }
}
