using MediatR;

namespace DomainService.Commands;

public sealed record LogoutUserCommand(string UserId) : IRequest<Unit>;
