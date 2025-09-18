using DomainService.Domain.Commands;
using DomainService.Interfaces;
using MediatR;
using System.Text.Json;
using System.Text.Json.Nodes;

namespace DomainService.Domain.CommandHandlers;

internal sealed class UpdateTask(ITaskEventRepository taskRepo, IEventQueue eventQueue) : ICommandHandler<UpdateTaskCommand>
{
    private readonly ITaskEventRepository _taskRepo = taskRepo;
    private readonly IEventQueue _eventQueue = eventQueue;

    public async Task<Unit> Handle(UpdateTaskCommand request, CancellationToken ct)
    {
        var events = await _taskRepo.Get(request.TaskId, ct);
        var state = TaskStateBuilder.From(events);
        if (state.Title == null) return Unit.Value;

        JsonElement? data = null;
        if (request.Data.HasValue)
        {
            var obj = JsonNode.Parse(request.Data.Value.GetRawText())?.AsObject();
            if (obj != null)
            {
                obj.Remove("id");

                if (state.Done &&
                    obj.TryGetPropertyValue("category", out var categoryNode) &&
                    categoryNode?.GetValue<string?>() is { } category &&
                    !string.Equals(category, "done", StringComparison.OrdinalIgnoreCase))
                {
                    obj["done"] = false;
                }

                data = obj.Count > 0 ? JsonSerializer.SerializeToElement(obj) : null;
            }
        }

        var ev = new Event(Guid.NewGuid().ToString(), request.TaskId, EntityTypes.Task, TaskEventTypes.Updated, data, request.Timestamp, request.UserId, request.IdempotencyKey);
        await _taskRepo.Add(ev, ct);
        await _eventQueue.Add(ev, ct);
        return Unit.Value;
    }
}
