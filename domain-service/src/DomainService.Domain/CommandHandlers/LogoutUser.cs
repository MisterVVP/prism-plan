using DomainService.Domain.Commands;
using DomainService.Interfaces;
using MediatR;

namespace DomainService.Domain.CommandHandlers;

internal sealed class LogoutUser(IUserEventRepository userRepo, IEventQueue eventQueue) : ICommandHandler<LogoutUserCommand>
{
    private readonly IUserEventRepository _userRepo = userRepo;
    private readonly IEventQueue _eventQueue = eventQueue;

    public async Task<Unit> Handle(LogoutUserCommand request, CancellationToken ct)
    {
        var ev = new Event(Guid.NewGuid().ToString(), request.UserId, EntityTypes.User, UserEventTypes.Logout, null, request.Timestamp, request.UserId, request.IdempotencyKey);
        await _userRepo.Add(ev, ct);
        await _eventQueue.Add(ev, ct);
        return Unit.Value;
    }
}
