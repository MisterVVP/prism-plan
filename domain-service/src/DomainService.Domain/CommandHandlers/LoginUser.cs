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
        var type = exists ? "user-logged-in" : "user-created";
        JsonElement? data = null;
        if (!exists)
        {
            data = JsonSerializer.SerializeToElement(new { name = request.Name, email = request.Email });
        }
        var ev = new Event(
            Guid.NewGuid().ToString(),
            request.UserId,
            "user",
            type,
            data,
            DateTimeOffset.UtcNow.ToUnixTimeMilliseconds(),
            request.UserId);
        await _userRepo.Add(ev, ct);
        await _eventQueue.Add(ev, ct);
        if (!exists)
        {
            var settingsData = JsonSerializer.SerializeToElement(new { tasksPerCategory = 3, displayDoneTasks = false });
            var settingsEv = new Event(
                Guid.NewGuid().ToString(),
                request.UserId,
                EntityTypes.UserSettings,
                "user-settings-created",
                settingsData,
                DateTimeOffset.UtcNow.ToUnixTimeMilliseconds(),
                request.UserId);
            await _userRepo.Add(settingsEv, ct);
            await _eventQueue.Add(settingsEv, ct);
        }
        return Unit.Value;
    }
}
