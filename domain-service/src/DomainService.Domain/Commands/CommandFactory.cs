using DomainService.Interfaces;
using Microsoft.Extensions.Logging;
using System.Text.Json;

namespace DomainService.Domain.Commands
{
    internal class CommandFactory(ILoggerFactory loggerFactory) : ICommandFactory
    {
        private readonly JsonSerializerOptions _jsonSerializerOptions = new() { PropertyNameCaseInsensitive = true };
        private readonly ILogger _logger = loggerFactory.CreateLogger<CommandFactory>();

        public ICommand Create(string queueMessage) 
        {
            try
            {
                var envelope = JsonSerializer.Deserialize<CommandEnvelope>(queueMessage, _jsonSerializerOptions) ?? throw new ArgumentNullException(nameof(queueMessage), "Invalid queueMessage! JSON content is null");
                ICommand cmd = envelope.Command.EntityType switch
                {
                    EntityTypes.Task => envelope.Command.Type switch
                    {
                        CommandTypes.CreateTask => new CreateTaskCommand(
                            envelope.Command.Data,
                            envelope.UserId,
                            envelope.Command.Timestamp,
                            envelope.Command.Id),
                        CommandTypes.UpdateTask => new UpdateTaskCommand(
                            envelope.Command.Data?.GetProperty("id").GetString() ?? string.Empty,
                            envelope.Command.Data,
                            envelope.UserId,
                            envelope.Command.Timestamp,
                            envelope.Command.Id),
                        CommandTypes.CompleteTask => new CompleteTaskCommand(
                            envelope.Command.Data?.GetProperty("id").GetString() ?? string.Empty,
                            envelope.UserId,
                            envelope.Command.Timestamp,
                            envelope.Command.Id),
                        CommandTypes.ReopenTask => new ReopenTaskCommand(
                            envelope.Command.Data?.GetProperty("id").GetString() ?? string.Empty,
                            envelope.UserId,
                            envelope.Command.Timestamp,
                            envelope.Command.Id),
                        _ => throw new ArgumentException("Unknown command type!", nameof(queueMessage))
                    },
                    EntityTypes.User => envelope.Command.Type switch
                    {
                        CommandTypes.LoginUser => new LoginUserCommand(
                            envelope.UserId,
                            envelope.Command.Data?.GetProperty("name").GetString() ?? string.Empty,
                            envelope.Command.Data?.GetProperty("email").GetString() ?? string.Empty,
                            envelope.Command.Timestamp,
                            envelope.Command.Id),
                        CommandTypes.LogoutUser => new LogoutUserCommand(envelope.UserId, envelope.Command.Timestamp, envelope.Command.Id),
                        _ => throw new ArgumentException("Unknown Command.Type!", nameof(queueMessage))
                    },
                    EntityTypes.UserSettings => envelope.Command.Type switch
                    {
                        CommandTypes.UpdateUserSettings => new UpdateUserSettingsCommand(envelope.Command.Data, envelope.UserId, envelope.Command.Timestamp, envelope.Command.Id),
                        _ => throw new ArgumentException("Unknown Command.Type!", nameof(queueMessage))
                    },
                    _ => throw new ArgumentException("Unknown Command.EntityType!", nameof(queueMessage))
                };
                _logger.LogDebug("Created {cmd} command", envelope.Command.Type);
                return cmd;
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, "Unable to create command instance");
                throw;
            }
        }
    }
}
