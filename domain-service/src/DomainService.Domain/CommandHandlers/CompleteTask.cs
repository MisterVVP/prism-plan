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
        var status = await _taskRepo.GetStatus(request.TaskId, ct);
        if (!status.Exists || status.Done) return Unit.Value;

        var ev = new Event(Guid.NewGuid().ToString(), request.TaskId, EntityTypes.Task, TaskEventTypes.Completed, null, request.Timestamp, request.UserId, request.IdempotencyKey);
        await _taskRepo.Add(ev, ct);
        await _eventQueue.Add(ev, ct);
        return Unit.Value;
    }
}
