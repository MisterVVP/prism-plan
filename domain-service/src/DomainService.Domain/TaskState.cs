using DomainService.Interfaces;

namespace DomainService.Domain;

internal sealed class TaskState
{
    public string? Title { get; set; }
    public string? Notes { get; set; }
    public string? Category { get; set; }
    public int Order { get; set; }
    public bool Done { get; set; }
}

internal static class TaskStateBuilder
{
    public static TaskState From(IEnumerable<IEvent> events)
    {
        var state = new TaskState();
        foreach (var ev in events)
        {
            Apply(state, ev);
        }
        return state;
    }

    private static void Apply(TaskState state, IEvent ev)
    {
        switch (ev.Type)
        {
            case TaskEventTypes.Created:
                if (ev.Data.HasValue)
                {
                    var data = ev.Data.Value;
                    state.Title = data.GetProperty("title").GetString();
                    if (data.TryGetProperty("notes", out var n)) state.Notes = n.GetString();
                    if (data.TryGetProperty("category", out var c)) state.Category = c.GetString();
                    if (data.TryGetProperty("order", out var o) && o.TryGetInt32(out var oi))
                    {
                        state.Order = oi;
                    }
                }
                break;
            case TaskEventTypes.Updated:
                if (ev.Data.HasValue)
                {
                    var data = ev.Data.Value;
                    if (data.TryGetProperty("title", out var t)) state.Title = t.GetString();
                    if (data.TryGetProperty("notes", out var n)) state.Notes = n.GetString();
                    if (data.TryGetProperty("category", out var c)) state.Category = c.GetString();
                    if (data.TryGetProperty("order", out var o) && o.TryGetInt32(out var oi)) state.Order = oi;
                    if (data.TryGetProperty("done", out var d) && d.ValueKind == System.Text.Json.JsonValueKind.False)
                    {
                        state.Done = false;
                    }
                }
                break;
            case TaskEventTypes.Completed:
                state.Done = true;
                break;
        }
    }
}
