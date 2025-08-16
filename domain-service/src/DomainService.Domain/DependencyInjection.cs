using DomainService.Domain.Commands;
using DomainService.Interfaces;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Logging;
using System.Reflection;

namespace DomainService.Domain;

public static class DependencyInjection
{
    public static IServiceCollection AddCommands(this IServiceCollection services)
    {
        services.AddMediatR(cfg => cfg.RegisterServicesFromAssembly(Assembly.GetExecutingAssembly()));
        services.AddSingleton<ICommandFactory>(cfg => new CommandFactory(cfg.GetRequiredService<ILoggerFactory>()));
        return services;
    }
}
