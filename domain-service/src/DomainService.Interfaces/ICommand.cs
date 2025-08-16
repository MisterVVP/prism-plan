using MediatR;

namespace DomainService.Interfaces;

public interface ICommand { }

public interface ICommand<out TResponse> : IRequest<TResponse>, ICommand;
