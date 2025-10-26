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
        var start = await _taskRepo.TryStartProcessing(request.IdempotencyKey, ct);
        if (start == IdempotencyResult.AlreadyProcessed)
        {
            await _taskRepo.ReplayStoredEvents(_dispatcher, request.IdempotencyKey, ct);
            return Unit.Value;
        }

        if (start == IdempotencyResult.InProgress)
        {
            return Unit.Value;
        }

        try
        {
            if (await _taskRepo.ReplayStoredEvents(_dispatcher, request.IdempotencyKey, ct))
            {
                await _taskRepo.MarkProcessingSucceeded(request.IdempotencyKey, ct);
                return Unit.Value;
            }

            var events = await _taskRepo.Get(request.TaskId, ct);
            var state = TaskStateBuilder.From(events);
            if (state.Title == null || state.Done)
            {
                await _taskRepo.MarkProcessingSucceeded(request.IdempotencyKey, ct);
                return Unit.Value;
            }

            var ev = new Event(Guid.NewGuid().ToString(), request.TaskId, EntityTypes.Task, TaskEventTypes.Completed, null, request.Timestamp, request.UserId, request.IdempotencyKey);
            await _taskRepo.Add(ev, ct);
            await _dispatcher.Dispatch(ev, ct);
            await _taskRepo.MarkAsDispatched(ev, ct);
            await _taskRepo.MarkProcessingSucceeded(request.IdempotencyKey, ct);
            return Unit.Value;
        }
        catch
        {
            await _taskRepo.MarkProcessingFailed(request.IdempotencyKey, ct);
            throw;
        }
    }
}
