using DomainService.Domain.Commands;
using DomainService.Interfaces;
using MediatR;
using System.Text.Json;

namespace DomainService.Domain.CommandHandlers;

internal sealed class UpdateUserSettings(IUserEventRepository userRepo, IEventQueue eventQueue) : ICommandHandler<UpdateUserSettingsCommand>
{
    private readonly IUserEventRepository _userRepo = userRepo;
    private readonly IEventQueue _eventQueue = eventQueue;

    public async Task<Unit> Handle(UpdateUserSettingsCommand request, CancellationToken ct)
    {
        var ev = new Event(
            Guid.NewGuid().ToString(),
            request.UserId,
            EntityTypes.UserSettings,
            "user-settings-updated",
            request.Data,
            DateTimeOffset.UtcNow.ToUnixTimeMilliseconds(),
            request.UserId);
        await _userRepo.Add(ev, ct);
        await _eventQueue.Add(ev, ct);
        return Unit.Value;
    }
}
