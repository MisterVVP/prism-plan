using DomainService;
using DomainService.Commands;
using DomainService.Interfaces;
using MediatR;

namespace DomainService.Handlers;

internal sealed class CompleteTaskCommandHandler : ICommandHandler<CompleteTaskCommand>
{
    private readonly ITaskEventRepository _taskRepo;
    private readonly IEventQueue _eventQueue;

    public CompleteTaskCommandHandler(ITaskEventRepository taskRepo, IEventQueue eventQueue)
    {
        _taskRepo = taskRepo;
        _eventQueue = eventQueue;
    }

    public async Task<Unit> Handle(CompleteTaskCommand request, CancellationToken ct)
    {
        var events = await _taskRepo.Get(request.TaskId, ct);
        var state = TaskStateBuilder.From(events);
        if (state.Title == null || state.Done) return Unit.Value;

        var ev = new Event(Guid.NewGuid().ToString(), request.TaskId, "task", "task-completed", null, DateTimeOffset.UtcNow.ToUnixTimeMilliseconds(), request.UserId);
        await _taskRepo.Add(ev, ct);
        await _eventQueue.Add(ev, ct);
        return Unit.Value;
    }
}
