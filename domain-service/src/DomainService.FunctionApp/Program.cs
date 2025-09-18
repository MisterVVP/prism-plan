using Azure.Core;
using Azure.Data.Tables;
using Azure.Storage.Queues;
using DomainService.Domain;
using DomainService.Interfaces;
using DomainService.Repositories;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using DomainService.Services;

var connStr = Environment.GetEnvironmentVariable("STORAGE_CONNECTION_STRING")
    ?? throw new InvalidOperationException("missing STORAGE_CONNECTION_STRING");
var domainEventsQueueName = Environment.GetEnvironmentVariable("DOMAIN_EVENTS_QUEUE")
    ?? throw new InvalidOperationException("missing DOMAIN_EVENTS_QUEUE");
var taskEventTableName = Environment.GetEnvironmentVariable("TASK_EVENTS_TABLE")
    ?? throw new InvalidOperationException("missing TASK_EVENTS_TABLE");
var userEventTableName = Environment.GetEnvironmentVariable("USER_EVENTS_TABLE")
    ?? throw new InvalidOperationException("missing USER_EVENTS_TABLE");
var readModelUpdaterUrl = Environment.GetEnvironmentVariable("READ_MODEL_UPDATER_URL")
    ?? throw new InvalidOperationException("missing READ_MODEL_UPDATER_URL");

var host = new HostBuilder()
    .ConfigureFunctionsWorkerDefaults()
    .ConfigureServices(services =>
    {
        var queueClientOptions = new QueueClientOptions
        {
            Retry = {
                Delay = TimeSpan.FromSeconds(1),
                MaxRetries = 5,
                Mode = RetryMode.Exponential,
                MaxDelay = TimeSpan.FromSeconds(15),
                NetworkTimeout = TimeSpan.FromSeconds(60)
            },
        };
        services.AddSingleton<IEventQueue>(_ => new StorageQueueEventQueue(new QueueClient(connStr, domainEventsQueueName, queueClientOptions)));
        var tableClientOptions = new TableClientOptions
        {
            Retry = {
                Delay = TimeSpan.FromSeconds(1),
                MaxRetries = 3,
                Mode = RetryMode.Exponential,
                MaxDelay = TimeSpan.FromSeconds(15),
                NetworkTimeout = TimeSpan.FromSeconds(60)
            },
        };
        services.AddSingleton<ITaskEventRepository>(_ => new TableTaskEventRepository(new TableClient(connStr, taskEventTableName, tableClientOptions)));
        services.AddSingleton<IUserEventRepository>(_ => new TableUserEventRepository(new TableClient(connStr, userEventTableName, tableClientOptions)));
        services.AddHttpClient<IReadModelUpdaterClient, HttpReadModelUpdaterClient>(client =>
        {
            client.BaseAddress = new Uri(readModelUpdaterUrl, UriKind.Absolute);
            client.Timeout = TimeSpan.FromSeconds(30);
        });
        services.AddSingleton<IEventDispatcher>(sp => new ResilientEventDispatcher(
            sp.GetRequiredService<IEventQueue>(),
            sp.GetRequiredService<IReadModelUpdaterClient>(),
            sp.GetRequiredService<Microsoft.Extensions.Logging.ILogger<ResilientEventDispatcher>>()));
        services.AddMediatR(cfg => cfg.RegisterServicesFromAssembly(typeof(Program).Assembly));
        services.AddCommands();
    })
    .Build();

host.Run();
