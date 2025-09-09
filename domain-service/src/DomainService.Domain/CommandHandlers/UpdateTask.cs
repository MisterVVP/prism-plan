using DomainService.Domain.Commands;
using DomainService.Interfaces;
using MediatR;
using System.Text.Json;
using System.Text.Json.Nodes;
using DomainService.Domain;

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

        JsonElement? data = request.Data;
        if (state.Done && request.Data.HasValue &&
            request.Data.Value.TryGetProperty("category", out var c) &&
            c.GetString() != null && !string.Equals(c.GetString(), "done", StringComparison.OrdinalIgnoreCase))
        {
            var obj = JsonNode.Parse(request.Data.Value.GetRawText())!.AsObject();
            obj["done"] = false;
            data = JsonSerializer.SerializeToElement(obj);
        }

        var ev = new Event(Guid.NewGuid().ToString(), request.TaskId, EntityTypes.Task, TaskEventTypes.Updated, data, request.Timestamp, request.UserId, request.IdempotencyKey);
        await _taskRepo.Add(ev, ct);
        await _eventQueue.Add(ev, ct);
        return Unit.Value;
    }
}
