using MediatR;

namespace DomainService.Commands;

public sealed record LoginUserCommand(string UserId) : IRequest<Unit>;
