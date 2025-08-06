using Azure.Data.Tables;
using Azure.Storage.Queues;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;
using System.Text.Json;

var connStr = Environment.GetEnvironmentVariable("STORAGE_CONNECTION_STRING")
    ?? throw new InvalidOperationException("missing STORAGE_CONNECTION_STRING");
var commandQueueName = Environment.GetEnvironmentVariable("COMMAND_QUEUE")
    ?? throw new InvalidOperationException("missing COMMAND_QUEUE");
var domainEventsQueueName = Environment.GetEnvironmentVariable("DOMAIN_EVENTS_QUEUE")
    ?? throw new InvalidOperationException("missing DOMAIN_EVENTS_QUEUE");
var eventTableName = Environment.GetEnvironmentVariable("TASK_EVENTS_TABLE")
    ?? throw new InvalidOperationException("missing TASK_EVENTS_TABLE");

var host = Host.CreateDefaultBuilder(args)
    .ConfigureServices(services =>
    {
        services.AddSingleton(new QueueClient(connStr, commandQueueName));
        services.AddSingleton(new QueueClient(connStr, domainEventsQueueName));
        services.AddSingleton(new TableClient(connStr, eventTableName));
        services.AddHostedService<Worker>();
    })
    .Build();

await host.RunAsync();

record Command(string Id, string EntityId, string EntityType, string Type, JsonElement? Data);
record CommandEnvelope(string UserId, Command Command);
record Event(string Id, string EntityId, string EntityType, string Type, JsonElement? Data, long Time, string UserId);

class TaskState
{
    public string? Title { get; set; }
    public string? Notes { get; set; }
    public string? Category { get; set; }
    public int Order { get; set; }
    public bool Done { get; set; }
}

class Worker : BackgroundService
{
    private readonly QueueClient _commandQueue;
    private readonly QueueClient _domainEventsQueue;
    private readonly TableClient _eventTable;
    private readonly ILogger<Worker> _logger;

    public Worker(QueueClient commandQueue, QueueClient domainEventsQueue, TableClient eventTable, ILogger<Worker> logger)
    {
        _commandQueue = commandQueue;
        _domainEventsQueue = domainEventsQueue;
        _eventTable = eventTable;
        _logger = logger;
    }

    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        _logger.LogInformation("Domain Service running");
        await _commandQueue.CreateIfNotExistsAsync();
        await _domainEventsQueue.CreateIfNotExistsAsync();
        await _eventTable.CreateIfNotExistsAsync();

        while (!stoppingToken.IsCancellationRequested)
        {
            var msg = await _commandQueue.ReceiveMessageAsync(cancellationToken: stoppingToken);
            if (msg.Value?.MessageText == null)
            {
                await Task.Delay(TimeSpan.FromSeconds(1), stoppingToken);
                continue;
            }

            try
            {
                var env = JsonSerializer.Deserialize<CommandEnvelope>(msg.Value.MessageText);
                if (env != null)
                {
                    await ProcessCommand(env, stoppingToken);
                }
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, "processing command");
            }

            await _commandQueue.DeleteMessageAsync(msg.Value.MessageId, msg.Value.PopReceipt, stoppingToken);
        }
    }

    private async Task ProcessCommand(CommandEnvelope env, CancellationToken ct)
    {
        if (env.Command.EntityType != "task") return;

        var state = new TaskState();
        var filter = $"PartitionKey eq '{env.Command.EntityId}'";
        await foreach (var e in _eventTable.QueryAsync<TableEntity>(filter: filter, cancellationToken: ct))
        {
            if (e.TryGetValue("Data", out object? dataObj) && dataObj is string data)
            {
                var ev = JsonSerializer.Deserialize<Event>(data);
                if (ev != null)
                {
                    Apply(state, ev);
                }
            }
        }

        Event? newEvent = env.Command.Type switch
        {
            "create-task" when state.Title == null => new Event(Guid.NewGuid().ToString(), env.Command.EntityId, "task", "task-created", env.Command.Data, DateTimeOffset.UtcNow.ToUnixTimeMilliseconds(), env.UserId),
            "update-task" when state.Title != null => new Event(Guid.NewGuid().ToString(), env.Command.EntityId, "task", "task-updated", env.Command.Data, DateTimeOffset.UtcNow.ToUnixTimeMilliseconds(), env.UserId),
            "complete-task" when state.Title != null && !state.Done => new Event(Guid.NewGuid().ToString(), env.Command.EntityId, "task", "task-completed", env.Command.Data, DateTimeOffset.UtcNow.ToUnixTimeMilliseconds(), env.UserId),
            _ => null
        };

        if (newEvent == null) return;

        var entity = new TableEntity(env.Command.EntityId, newEvent.Id)
        {
            { "UserId", env.UserId },
            { "Data", JsonSerializer.Serialize(newEvent) }
        };
        await _eventTable.AddEntityAsync(entity, ct);
        await _domainEventsQueue.SendMessageAsync(JsonSerializer.Serialize(newEvent), cancellationToken: ct);
    }

    private static void Apply(TaskState state, Event ev)
    {
        switch (ev.Type)
        {
            case "task-created":
                if (ev.Data.HasValue)
                {
                    var data = ev.Data.Value;
                    state.Title = data.GetProperty("title").GetString();
                    state.Notes = data.GetProperty("notes").GetString();
                    state.Category = data.GetProperty("category").GetString();
                    if (data.TryGetProperty("order", out var o) && o.TryGetInt32(out var oi))
                    {
                        state.Order = oi;
                    }
                }
                break;
            case "task-updated":
                if (ev.Data.HasValue)
                {
                    var data = ev.Data.Value;
                    if (data.TryGetProperty("title", out var t)) state.Title = t.GetString();
                    if (data.TryGetProperty("notes", out var n)) state.Notes = n.GetString();
                    if (data.TryGetProperty("category", out var c)) state.Category = c.GetString();
                    if (data.TryGetProperty("order", out var o) && o.TryGetInt32(out var oi)) state.Order = oi;
                }
                break;
            case "task-completed":
                state.Done = true;
                break;
        }
    }
}
