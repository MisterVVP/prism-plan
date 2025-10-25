using DomainService.Domain.Commands;
using DomainService.Interfaces;
using MediatR;

namespace DomainService.Domain.CommandHandlers;

internal sealed class CompleteTask(ITaskEventRepository taskRepo, IEventDispatcher dispatcher) : ICommandHandler<CompleteTaskCommand>
{
    private readonly ITaskEventRepository _taskRepo = taskRepo;
    private readonly IEventDispatcher _dispatcher = dispatcher;

    public async Task<Unit> Handle(CompleteTaskCommand request, CancellationToken ct)
    {
        if (await _taskRepo.ReplayStoredEvents(_dispatcher, request.IdempotencyKey, ct))
        {
            return Unit.Value;
        }

        var events = await _taskRepo.Get(request.TaskId, ct);
        var state = TaskStateBuilder.From(events);
        if (state.Title == null || state.Done) return Unit.Value;

        var ev = new Event(Guid.NewGuid().ToString(), request.TaskId, EntityTypes.Task, TaskEventTypes.Completed, null, request.Timestamp, request.UserId, request.IdempotencyKey);
        await _taskRepo.Add(ev, ct);
        await _dispatcher.Dispatch(ev, ct);
        await _taskRepo.MarkAsDispatched(ev, ct);
        return Unit.Value;
    }
}
