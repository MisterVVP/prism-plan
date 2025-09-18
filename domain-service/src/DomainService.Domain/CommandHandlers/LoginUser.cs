using DomainService.Domain.Commands;
using DomainService.Interfaces;
using MediatR;
using System.Text.Json;
using DomainService.Domain;

namespace DomainService.Domain.CommandHandlers;

internal sealed class LoginUser(IUserEventRepository userRepo, IEventDispatcher dispatcher) : ICommandHandler<LoginUserCommand>
{
    private readonly IUserEventRepository _userRepo = userRepo;
    private readonly IEventDispatcher _dispatcher = dispatcher;

    public async Task<Unit> Handle(LoginUserCommand request, CancellationToken ct)
    {
        if (await _userRepo.ReplayStoredEvents(_dispatcher, request.IdempotencyKey, ct))
        {
            return Unit.Value;
        }

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
        await _dispatcher.Dispatch(ev, ct);
        await _userRepo.MarkAsDispatched(ev, ct);
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
            await _dispatcher.Dispatch(settingsEv, ct);
            await _userRepo.MarkAsDispatched(settingsEv, ct);
        }
        return Unit.Value;
    }
}
