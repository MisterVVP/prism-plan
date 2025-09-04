using DomainService.Domain.CommandHandlers;
using DomainService.Domain.Commands;
using DomainService.Interfaces;
using System.Text.Json;
using Xunit;

public class HandlersTests
{
    [Fact]
    public async Task CreateTask_adds_event()
    {
        var repo = new InMemoryTaskRepo();
        var queue = new InMemoryQueue();
        ICommandHandler<CreateTaskCommand> handler = new CreateTask(repo, queue);
        var cmd = new CreateTaskCommand(JsonDocument.Parse("{\"title\":\"t\",\"notes\":\"n\",\"category\":\"c\"}").RootElement, "u1", 1);
        await handler.Handle(cmd, CancellationToken.None);
        Assert.Single(repo.Events);
        Assert.Single(queue.Events);
        Assert.Equal("task-created", repo.Events[0].Type);
        Assert.False(string.IsNullOrWhiteSpace(repo.Events[0].EntityId));
    }

    [Fact]
    public async Task UpdateTask_adds_event_when_exists()
    {
        var repo = new InMemoryTaskRepo();
        var queue = new InMemoryQueue();
        var seed = new Event("e1", "t1", "task", "task-created", JsonDocument.Parse("{\"title\":\"t\"}").RootElement, 0, "u1");
        await repo.Add(seed, CancellationToken.None);
        ICommandHandler<UpdateTaskCommand> handler = new UpdateTask(repo, queue);
        var cmd = new UpdateTaskCommand("t1", JsonDocument.Parse("{\"notes\":\"n\"}").RootElement, "u1", 1);
        await handler.Handle(cmd, CancellationToken.None);
        Assert.Equal(2, repo.Events.Count);
        Assert.Equal("task-updated", repo.Events[1].Type);
    }

    [Fact]
    public async Task CompleteTask_adds_event_when_not_done()
    {
        var repo = new InMemoryTaskRepo();
        var queue = new InMemoryQueue();
        var seed = new Event("e1", "t1", "task", "task-created", JsonDocument.Parse("{\"title\":\"t\"}").RootElement, 0, "u1");
        await repo.Add(seed, CancellationToken.None);
        ICommandHandler<CompleteTaskCommand> handler = new CompleteTask(repo, queue);
        var cmd = new CompleteTaskCommand("t1", "u1", 1);
        await handler.Handle(cmd, CancellationToken.None);
        Assert.Equal(2, repo.Events.Count);
        Assert.Equal("task-completed", repo.Events[1].Type);
    }

    [Fact]
    public async Task LoginUser_logs_in_existing_user()
    {
        var repo = new InMemoryUserRepo();
        var queue = new InMemoryQueue();
        var seed = new Event("e1", "u1", "user", "user-created", null, 0, "u1");
        await repo.Add(seed, CancellationToken.None);
        ICommandHandler<LoginUserCommand> handler = new LoginUser(repo, queue);
        var cmd = new LoginUserCommand("u1", "n", "e", 1);
        await handler.Handle(cmd, CancellationToken.None);
        Assert.Equal(2, repo.Events.Count);
        Assert.Equal("user-logged-in", repo.Events[1].Type);
    }

    [Fact]
    public async Task LogoutUser_enqueues_event()
    {
        var repo = new InMemoryUserRepo();
        var queue = new InMemoryQueue();
        ICommandHandler<LogoutUserCommand> handler = new LogoutUser(repo, queue);
        var cmd = new LogoutUserCommand("u1", 1);
        await handler.Handle(cmd, CancellationToken.None);
        Assert.Single(repo.Events);
        Assert.Equal("user-logged-out", repo.Events[0].Type);
    }

    [Fact]
    public async Task UpdateUserSettings_adds_event()
    {
        var repo = new InMemoryUserRepo();
        var queue = new InMemoryQueue();
        ICommandHandler<UpdateUserSettingsCommand> handler = new UpdateUserSettings(repo, queue);
        var cmd = new UpdateUserSettingsCommand(JsonDocument.Parse("{\"tasksPerCategory\":5}").RootElement, "u1", 1);
        await handler.Handle(cmd, CancellationToken.None);
        Assert.Single(repo.Events);
        Assert.Equal("user-settings-updated", repo.Events[0].Type);
    }
}

class InMemoryQueue : IEventQueue
{
    public List<IEvent> Events { get; } = [];
    public Task Add(IEvent ev, CancellationToken ct)
    {
        Events.Add(ev);
        return Task.CompletedTask;
    }
}

class InMemoryTaskRepo : ITaskEventRepository
{
    public List<IEvent> Events { get; } = [];
    public Task Add(IEvent ev, CancellationToken ct)
    {
        Events.Add(ev);
        return Task.CompletedTask;
    }
    public Task<IReadOnlyList<IEvent>> Get(string taskId, CancellationToken ct)
    {
        return Task.FromResult<IReadOnlyList<IEvent>>([.. Events.Where(e => e.EntityId == taskId)]);
    }
}

class InMemoryUserRepo : IUserEventRepository
{
    public List<IEvent> Events { get; } = [];
    public Task Add(IEvent ev, CancellationToken ct)
    {
        Events.Add(ev);
        return Task.CompletedTask;
    }
    public Task<bool> Exists(string userId, CancellationToken ct)
    {
        return Task.FromResult(Events.Any(e => e.EntityId == userId));
    }
}
