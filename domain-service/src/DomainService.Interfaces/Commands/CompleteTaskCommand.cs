using MediatR;

namespace DomainService.Commands;

public sealed record CompleteTaskCommand(string TaskId, string UserId) : IRequest<Unit>;
