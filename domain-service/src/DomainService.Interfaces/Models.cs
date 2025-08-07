using System.Text.Json;
using DomainService.Interfaces;

namespace DomainService;

public sealed record Command(string Id, string EntityId, string EntityType, string Type, JsonElement? Data);
public sealed record CommandEnvelope(string UserId, Command Command);
public sealed record Event(string Id, string EntityId, string EntityType, string Type, JsonElement? Data, long Time, string UserId) : IEvent;
