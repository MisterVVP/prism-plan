using System.Text.Json;
using MediatR;

namespace DomainService.Commands;

public sealed record UpdateTaskCommand(string TaskId, JsonElement? Data, string UserId) : IRequest<Unit>;
