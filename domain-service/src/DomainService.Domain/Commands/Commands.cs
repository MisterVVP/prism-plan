using DomainService.Interfaces;
using MediatR;
using System.Text.Json;

namespace DomainService.Domain.Commands
{
    public sealed record CompleteTaskCommand(string TaskId, string UserId) : ICommand<Unit>;

    public sealed record CreateTaskCommand(string TaskId, JsonElement? Data, string UserId) : ICommand<Unit>;

    public sealed record LoginUserCommand(string UserId, string Name, string Email) : ICommand<Unit>;

    public sealed record LogoutUserCommand(string UserId) : ICommand<Unit>;

    public sealed record UpdateTaskCommand(string TaskId, JsonElement? Data, string UserId) : ICommand<Unit>;

    public sealed record UpdateUserSettingsCommand(JsonElement? Data, string UserId) : ICommand<Unit>;

}
