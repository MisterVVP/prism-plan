using DomainService.Domain.Commands;
using DomainService.Interfaces;
using MediatR;
using System.Text.Json;

namespace DomainService.Domain.CommandHandlers;

internal sealed class LoginUser(IUserEventRepository userRepo, IEventQueue eventQueue) : ICommandHandler<LoginUserCommand>
{
    private readonly IUserEventRepository _userRepo = userRepo;
    private readonly IEventQueue _eventQueue = eventQueue;

    public async Task<Unit> Handle(LoginUserCommand request, CancellationToken ct)
    {
        var exists = await _userRepo.Exists(request.UserId, ct);
        var type = exists ? UserEventTypes.Login : UserEventTypes.Created;
        JsonElement? data = null;
        if (!exists)
        {
            data = JsonSerializer.SerializeToElement(new UserProfileData(request.Name, request.Email));
        }
        var ev = new Event(
            Guid.NewGuid().ToString(),
            request.UserId,
            EntityTypes.User,
            type,
            data,
            request.Timestamp,
            request.UserId,
            request.IdempotencyKey);
        await _userRepo.Add(ev, ct);
        await _eventQueue.Add(ev, ct);
        if (!exists)
        {
            var settingsData = JsonSerializer.SerializeToElement(new UserSettingsData(3, false));
            var settingsEv = new Event(
                Guid.NewGuid().ToString(),
                request.UserId,
                EntityTypes.UserSettings,
                UserEventTypes.SettingsCreated,
                settingsData,
                request.Timestamp,
                request.UserId,
                request.IdempotencyKey);
            await _userRepo.Add(settingsEv, ct);
            await _eventQueue.Add(settingsEv, ct);
        }
        return Unit.Value;
    }
}
