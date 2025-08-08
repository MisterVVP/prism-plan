using DomainService.Domain.Commands;
using DomainService.Interfaces;
using MediatR;
using Microsoft.Azure.Functions.Worker;
using Microsoft.Extensions.Logging;
using System.Text.Json;

namespace DomainService;

internal sealed class CommandQueueFunction(ISender sender, ILoggerFactory loggerFactory)
{
    private readonly ISender _sender = sender;
    private readonly ILogger _logger = loggerFactory.CreateLogger<CommandQueueFunction>();
    private readonly JsonSerializerOptions _jsonSerializerOptions = new() { PropertyNameCaseInsensitive = true };

    [Function("CommandQueueFunction")]
    public async Task Run([QueueTrigger("%COMMAND_QUEUE%", Connection = "STORAGE_CONNECTION_STRING")] string msg, FunctionContext context)
    {
        try
        {
            var env = JsonSerializer.Deserialize<CommandEnvelope>(msg, _jsonSerializerOptions);
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
                    "login-user" => new LoginUserCommand(
                        env.Command.EntityId,
                        env.Command.Data?.GetProperty("name").GetString() ?? string.Empty,
                        env.Command.Data?.GetProperty("email").GetString() ?? string.Empty),
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
