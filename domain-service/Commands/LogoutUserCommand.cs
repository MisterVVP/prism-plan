using MediatR;

namespace DomainService.Commands;

internal sealed record LogoutUserCommand(string UserId) : IRequest<Unit>;
