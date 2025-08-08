using DomainService.Domain.Commands;
using DomainService.Interfaces;
using MediatR;

namespace DomainService.Domain.CommandHandlers;

internal sealed class CompleteTask(ITaskEventRepository taskRepo, IEventQueue eventQueue) : ICommandHandler<CompleteTaskCommand>
{
    private readonly ITaskEventRepository _taskRepo = taskRepo;
    private readonly IEventQueue _eventQueue = eventQueue;

    public async Task<Unit> Handle(CompleteTaskCommand request, CancellationToken ct)
    {
        var events = await _taskRepo.Get(request.TaskId, ct);
        var state = TaskStateBuilder.From(events);
        if (state.Title == null || state.Done) return Unit.Value;

        var ev = new Event(Guid.NewGuid().ToString(), request.TaskId, EntityTypes.Task, TaskEventTypes.Completed, null, DateTimeOffset.UtcNow.ToUnixTimeMilliseconds(), request.UserId);
        await _taskRepo.Add(ev, ct);
        await _eventQueue.Add(ev, ct);
        return Unit.Value;
    }
}
