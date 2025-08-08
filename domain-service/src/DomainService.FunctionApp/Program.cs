using Azure.Data.Tables;
using Azure.Storage.Queues;
using DomainService.Domain;
using DomainService.Interfaces;
using DomainService.Repositories;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;

var connStr = Environment.GetEnvironmentVariable("STORAGE_CONNECTION_STRING")
    ?? throw new InvalidOperationException("missing STORAGE_CONNECTION_STRING");
var domainEventsQueueName = Environment.GetEnvironmentVariable("DOMAIN_EVENTS_QUEUE")
    ?? throw new InvalidOperationException("missing DOMAIN_EVENTS_QUEUE");
var taskEventTableName = Environment.GetEnvironmentVariable("TASK_EVENTS_TABLE")
    ?? throw new InvalidOperationException("missing TASK_EVENTS_TABLE");
var userEventTableName = Environment.GetEnvironmentVariable("USER_EVENTS_TABLE")
    ?? throw new InvalidOperationException("missing USER_EVENTS_TABLE");

var host = new HostBuilder()
    .ConfigureFunctionsWorkerDefaults()
    .ConfigureServices(services =>
    {
        services.AddSingleton<IEventQueue>(_ => new StorageQueueEventQueue(new QueueClient(connStr, domainEventsQueueName)));
        services.AddSingleton<ITaskEventRepository>(_ => new TableTaskEventRepository(new TableClient(connStr, taskEventTableName)));
        services.AddSingleton<IUserEventRepository>(_ => new TableUserEventRepository(new TableClient(connStr, userEventTableName)));
        services.AddMediatR(cfg => cfg.RegisterServicesFromAssembly(typeof(Program).Assembly));
        services.AddCommands();
    })
    .Build();

host.Run();
