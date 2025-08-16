using DomainService.Interfaces;
using MediatR;
using Microsoft.Azure.Functions.Worker;
using Microsoft.Extensions.Logging;

namespace DomainService;

internal sealed class CommandQueueFunction(ISender sender, ILoggerFactory loggerFactory, ICommandFactory commandFactory)
{
    private readonly ISender _sender = sender;
    private readonly ILogger _logger = loggerFactory.CreateLogger<CommandQueueFunction>();
    private readonly ICommandFactory _commandFactory = commandFactory;

    [Function("CommandQueueFunction")]
    public async Task Run([QueueTrigger("%COMMAND_QUEUE%", Connection = "STORAGE_CONNECTION_STRING")] string msg, FunctionContext context)
    {
        try
        {
            var command = _commandFactory.Create(msg);
            if (command != null) {
                await _sender.Send(command, context.CancellationToken);
            } else
            {
                _logger.LogDebug("Unable to create command from message: {msg}", msg);
            }
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "processing command");
        }
    }
}
