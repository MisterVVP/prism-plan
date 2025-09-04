using DomainService.Domain.Commands;
using DomainService.Interfaces;
using MediatR;

namespace DomainService.Domain.CommandHandlers;

internal sealed class CreateTask(ITaskEventRepository taskRepo, IEventQueue eventQueue) : ICommandHandler<CreateTaskCommand>
{
    private readonly ITaskEventRepository _taskRepo = taskRepo;
    private readonly IEventQueue _eventQueue = eventQueue;

    public async Task<Unit> Handle(CreateTaskCommand request, CancellationToken ct)
    {
        var taskId = Guid.NewGuid().ToString();
        var ev = new Event(Guid.NewGuid().ToString(), taskId, EntityTypes.Task, TaskEventTypes.Created, request.Data, request.Timestamp, request.UserId);
        await _taskRepo.Add(ev, ct);
        await _eventQueue.Add(ev, ct);
        return Unit.Value;
    }
}
