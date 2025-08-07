using MediatR;

namespace DomainService.Commands;

internal sealed record CompleteTaskCommand(string TaskId, string UserId) : IRequest<Unit>;
