using DomainService.Domain.Commands;
using DomainService.Interfaces;
using MediatR;

namespace DomainService.Domain.CommandHandlers;

internal sealed class UpdateUserSettings(IUserEventRepository userRepo, IEventDispatcher dispatcher) : ICommandHandler<UpdateUserSettingsCommand>
{
    private readonly IUserEventRepository _userRepo = userRepo;
    private readonly IEventDispatcher _dispatcher = dispatcher;

    public async Task<Unit> Handle(UpdateUserSettingsCommand request, CancellationToken ct)
    {
        var start = await _userRepo.TryStartProcessing(request.IdempotencyKey, ct);
        if (start == IdempotencyResult.AlreadyProcessed)
        {
            await _userRepo.ReplayStoredEvents(_dispatcher, request.IdempotencyKey, ct);
            return Unit.Value;
        }

        if (start == IdempotencyResult.InProgress)
        {
            return Unit.Value;
        }

        try
        {
            if (await _userRepo.ReplayStoredEvents(_dispatcher, request.IdempotencyKey, ct))
            {
                await _userRepo.MarkProcessingSucceeded(request.IdempotencyKey, ct);
                return Unit.Value;
            }

            var ev = new Event(
                Guid.NewGuid().ToString(),
                request.UserId,
                EntityTypes.UserSettings,
                UserEventTypes.SettingsUpdated,
                request.Data,
                request.Timestamp,
                request.UserId,
                request.IdempotencyKey);
            await _userRepo.Add(ev, ct);
            await _dispatcher.Dispatch(ev, ct);
            await _userRepo.MarkAsDispatched(ev, ct);
            await _userRepo.MarkProcessingSucceeded(request.IdempotencyKey, ct);
            return Unit.Value;
        }
        catch
        {
            await _userRepo.MarkProcessingFailed(request.IdempotencyKey, ct);
            throw;
        }
    }
}
