using System.Collections.Generic;
using DomainService;
using DomainService.Interfaces;
using MediatR;
using Microsoft.Azure.Functions.Worker;
using Microsoft.Extensions.Logging.Abstractions;
using Moq;
using Xunit;

namespace DomainService.Tests;

public class CommandQueueFunctionTests
{
    [Fact]
    public async Task Run_Propagates_Exception_From_Command_Handler()
    {
        var sender = new ThrowingSender();
        var commandFactory = new StubCommandFactory();
        var function = new CommandQueueFunction(sender, NullLoggerFactory.Instance, commandFactory);
        var context = new Mock<FunctionContext>();
        context.SetupGet(ctx => ctx.CancellationToken).Returns(CancellationToken.None);

        await Assert.ThrowsAsync<InvalidOperationException>(() => function.Run("{}", context.Object));
    }

    private sealed class ThrowingSender : ISender
    {
        Task ISender.Send<TRequest>(TRequest request, CancellationToken cancellationToken) =>
            throw new InvalidOperationException("Command handler failed.");

        public Task<TResponse> Send<TResponse>(IRequest<TResponse> request, CancellationToken cancellationToken) =>
            throw new InvalidOperationException("Command handler failed.");

        public Task<object?> Send(object request, CancellationToken cancellationToken) =>
            throw new InvalidOperationException("Command handler failed.");

        public IAsyncEnumerable<TResponse> CreateStream<TResponse>(IStreamRequest<TResponse> request, CancellationToken cancellationToken) =>
            throw new InvalidOperationException("Command handler failed.");

        public IAsyncEnumerable<object?> CreateStream(object request, CancellationToken cancellationToken) =>
            throw new InvalidOperationException("Command handler failed.");
    }

    private sealed class StubCommandFactory : ICommandFactory
    {
        public ICommand Create(string queueMessage) => new FakeCommand();

        private sealed record FakeCommand() : ICommand<Unit>;
    }
}
