using DomainService;
using DomainService.Commands;
using DomainService.Interfaces;
using MediatR;

namespace DomainService.Handlers;

internal sealed class LogoutUserCommandHandler : ICommandHandler<LogoutUserCommand>
{
    private readonly IUserEventRepository _userRepo;
    private readonly IEventQueue _eventQueue;

    public LogoutUserCommandHandler(IUserEventRepository userRepo, IEventQueue eventQueue)
    {
        _userRepo = userRepo;
        _eventQueue = eventQueue;
    }

    public async Task<Unit> Handle(LogoutUserCommand request, CancellationToken ct)
    {
        var ev = new Event(Guid.NewGuid().ToString(), request.UserId, "user", "user-logged-out", null, DateTimeOffset.UtcNow.ToUnixTimeMilliseconds(), request.UserId);
        await _userRepo.Add(ev, ct);
        await _eventQueue.Add(ev, ct);
        return Unit.Value;
    }
}
