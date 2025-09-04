using System.Text.Json;

namespace DomainService.Interfaces;

public sealed record Command(string Id, string EntityType, string Type, JsonElement? Data, long Timestamp);
public sealed record CommandEnvelope(string UserId, Command Command);
public sealed record Event(string Id, string EntityId, string EntityType, string Type, JsonElement? Data, long Timestamp, string UserId) : IEvent;
