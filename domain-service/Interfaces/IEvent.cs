using System.Text.Json;

namespace DomainService.Interfaces;

public interface IEvent
{
    string Id { get; }
    string EntityId { get; }
    string EntityType { get; }
    string Type { get; }
    JsonElement? Data { get; }
    long Time { get; }
    string UserId { get; }
}
