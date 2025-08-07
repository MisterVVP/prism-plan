using System.Text.Json;
using DomainService.Commands;
using MediatR;
using Microsoft.Azure.Functions.Worker;
using Microsoft.Extensions.Logging;

namespace DomainService;

internal sealed class CommandQueueFunction
{
    private readonly ISender _sender;
    private readonly ILogger _logger;

    public CommandQueueFunction(ISender sender, ILoggerFactory loggerFactory)
    {
        _sender = sender;
        _logger = loggerFactory.CreateLogger<CommandQueueFunction>();
    }

    [Function("CommandQueueFunction")]
    public async Task Run([QueueTrigger("%COMMAND_QUEUE%", Connection = "STORAGE_CONNECTION_STRING")] string msg, FunctionContext context)
    {
        try
        {
            var env = JsonSerializer.Deserialize<CommandEnvelope>(msg, new JsonSerializerOptions { PropertyNameCaseInsensitive = true });
            if (env == null) return;

            IRequest<Unit>? cmd = env.Command.EntityType switch
            {
                EntityTypes.Task => env.Command.Type switch
                {
                    "create-task" => new CreateTaskCommand(env.Command.EntityId, env.Command.Data, env.UserId),
                    "update-task" => new UpdateTaskCommand(env.Command.EntityId, env.Command.Data, env.UserId),
                    "complete-task" => new CompleteTaskCommand(env.Command.EntityId, env.UserId),
                    _ => null
                },
                "user" => env.Command.Type switch
                {
                    "login-user" => new LoginUserCommand(env.Command.EntityId),
                    "logout-user" => new LogoutUserCommand(env.Command.EntityId),
                    _ => null
                },
                _ => null
            };

            if (cmd != null)
            {
                await _sender.Send(cmd, context.CancellationToken);
            }
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "processing command");
        }
    }
}
