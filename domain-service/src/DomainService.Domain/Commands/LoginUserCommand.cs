using MediatR;

namespace DomainService.Commands;

internal sealed record LoginUserCommand(string UserId) : IRequest<Unit>;
