using MediatR;

namespace DomainService.Interfaces;

public interface ICommandHandler<in TCommand> : IRequestHandler<TCommand, Unit> where TCommand : IRequest<Unit>;
