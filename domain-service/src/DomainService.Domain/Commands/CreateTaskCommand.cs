using System.Text.Json;
using MediatR;

namespace DomainService.Commands;

internal sealed record CreateTaskCommand(string TaskId, JsonElement? Data, string UserId) : IRequest<Unit>;
