using DomainService.Domain.Commands;
using DomainService.Interfaces;
using MediatR;

namespace DomainService.Domain.CommandHandlers;

internal sealed class CreateTask(ITaskEventRepository taskRepo, IEventDispatcher dispatcher) : ICommandHandler<CreateTaskCommand>
{
    private readonly ITaskEventRepository _taskRepo = taskRepo;
    private readonly IEventDispatcher _dispatcher = dispatcher;

    public async Task<Unit> Handle(CreateTaskCommand request, CancellationToken ct)
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

            var taskId = Guid.NewGuid().ToString();
            var ev = new Event(Guid.NewGuid().ToString(), taskId, EntityTypes.Task, TaskEventTypes.Created, request.Data, request.Timestamp, request.UserId, request.IdempotencyKey);
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
