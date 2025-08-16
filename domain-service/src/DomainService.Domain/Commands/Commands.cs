using MediatR;
using System.Text.Json;

namespace DomainService.Domain.Commands
{
    public sealed record CompleteTaskCommand(string TaskId, string UserId) : IRequest<Unit>
    {
        public const string CommandType = "complete-task";
    }

    public sealed record CreateTaskCommand(string TaskId, JsonElement? Data, string UserId) : IRequest<Unit>
    {
        public const string CommandType = "create-task";
    }

    public sealed record LoginUserCommand(string UserId, string Name, string Email) : IRequest<Unit>
    {
        public const string CommandType = "login-user";
    }

    public sealed record LogoutUserCommand(string UserId) : IRequest<Unit>
    {
        public const string CommandType = "logout-user";
    }

    public sealed record UpdateTaskCommand(string TaskId, JsonElement? Data, string UserId) : IRequest<Unit>
    {
        public const string CommandType = "update-task";
    }
}
