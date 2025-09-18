using DomainService.Interfaces;
using System.Net.Http;
using System.Text;
using System.Text.Json;

namespace DomainService.Services;

internal sealed class HttpReadModelUpdaterClient(HttpClient client) : IReadModelUpdaterClient
{
    private readonly HttpClient _client = client;
    private static readonly JsonSerializerOptions SerializerOptions = new()
    {
        PropertyNamingPolicy = null
    };

    public async Task SendAsync(IEvent ev, CancellationToken ct)
    {
        var envelope = new
        {
            Data = new
            {
                Event = JsonSerializer.Serialize(ev, SerializerOptions)
            }
        };

        using var content = new StringContent(JsonSerializer.Serialize(envelope, SerializerOptions), Encoding.UTF8, "application/json");
        using var response = await _client.PostAsync("api/domain-events", content, ct);
        response.EnsureSuccessStatusCode();
    }
}
