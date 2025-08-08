
using MediatR;
using System.Text.Json;

namespace DomainService.Domain.Commands
{
    public sealed record CompleteTaskCommand(string TaskId, string UserId) : IRequest<Unit>;
    public sealed record CreateTaskCommand(string TaskId, JsonElement? Data, string UserId) : IRequest<Unit>;
    public sealed record LoginUserCommand(string UserId) : IRequest<Unit>;
    public sealed record LogoutUserCommand(string UserId) : IRequest<Unit>;
    public sealed record UpdateTaskCommand(string TaskId, JsonElement? Data, string UserId) : IRequest<Unit>;
}
