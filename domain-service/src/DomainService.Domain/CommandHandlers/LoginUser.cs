using DomainService.Domain.Commands;
using DomainService.Interfaces;
using MediatR;

namespace DomainService.Domain.CommandHandlers;

internal sealed class LoginUser(IUserEventRepository userRepo, IEventQueue eventQueue) : ICommandHandler<LoginUserCommand>
{
    private readonly IUserEventRepository _userRepo = userRepo;
    private readonly IEventQueue _eventQueue = eventQueue;

    public async Task<Unit> Handle(LoginUserCommand request, CancellationToken ct)
    {
        var exists = await _userRepo.Exists(request.UserId, ct);
        var type = exists ? "user-logged-in" : "user-created";
        var ev = new Event(Guid.NewGuid().ToString(), request.UserId, "user", type, null, DateTimeOffset.UtcNow.ToUnixTimeMilliseconds(), request.UserId);
        await _userRepo.Add(ev, ct);
        await _eventQueue.Add(ev, ct);
        return Unit.Value;
    }
}
