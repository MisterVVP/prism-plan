using DomainService.Domain.Commands;
using DomainService.Interfaces;
using MediatR;
using System.Text.Json;
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

        var ev = new Event(Guid.NewGuid().ToString(), request.TaskId, EntityTypes.Task, TaskEventTypes.Updated, request.Data, request.Timestamp, request.UserId);
        await _taskRepo.Add(ev, ct);
        await _eventQueue.Add(ev, ct);

        if (state.Done && request.Data.HasValue &&
            request.Data.Value.TryGetProperty("category", out var c) &&
            c.GetString() != null && !string.Equals(c.GetString(), "done", StringComparison.OrdinalIgnoreCase))
        {
            var reopen = new Event(
                Guid.NewGuid().ToString(),
                request.TaskId,
                EntityTypes.Task,
                TaskEventTypes.Updated,
                JsonSerializer.SerializeToElement(new TaskStatusData(false)),
                request.Timestamp,
                request.UserId);
            await _taskRepo.Add(reopen, ct);
            await _eventQueue.Add(reopen, ct);
        }
        return Unit.Value;
    }
}
