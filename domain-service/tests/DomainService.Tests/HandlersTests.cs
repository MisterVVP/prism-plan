using System.Collections.Generic;
using System.Linq;
using System.Text.Json;
using DomainService;
using DomainService.Commands;
using DomainService.Handlers;
using DomainService.Interfaces;
using Xunit;

public class HandlersTests
{
    [Fact]
    public async Task CreateTask_adds_event()
    {
        var repo = new InMemoryTaskRepo();
        var queue = new InMemoryQueue();
        ICommandHandler<CreateTaskCommand> handler = new CreateTaskCommandHandler(repo, queue);
        var cmd = new CreateTaskCommand("t1", JsonDocument.Parse("{\"title\":\"t\",\"notes\":\"n\",\"category\":\"c\"}").RootElement, "u1");
        await handler.Handle(cmd, CancellationToken.None);
        Assert.Single(repo.Events);
        Assert.Single(queue.Events);
        Assert.Equal("task-created", repo.Events[0].Type);
    }

    [Fact]
    public async Task UpdateTask_adds_event_when_exists()
    {
        var repo = new InMemoryTaskRepo();
        var queue = new InMemoryQueue();
        var seed = new Event("e1", "t1", "task", "task-created", JsonDocument.Parse("{\"title\":\"t\"}").RootElement, 0, "u1");
        await repo.Add(seed, CancellationToken.None);
        ICommandHandler<UpdateTaskCommand> handler = new UpdateTaskCommandHandler(repo, queue);
        var cmd = new UpdateTaskCommand("t1", JsonDocument.Parse("{\"notes\":\"n\"}").RootElement, "u1");
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
        ICommandHandler<CompleteTaskCommand> handler = new CompleteTaskCommandHandler(repo, queue);
        var cmd = new CompleteTaskCommand("t1", "u1");
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
        ICommandHandler<LoginUserCommand> handler = new LoginUserCommandHandler(repo, queue);
        var cmd = new LoginUserCommand("u1");
        await handler.Handle(cmd, CancellationToken.None);
        Assert.Equal(2, repo.Events.Count);
        Assert.Equal("user-logged-in", repo.Events[1].Type);
    }

    [Fact]
    public async Task LogoutUser_enqueues_event()
    {
        var repo = new InMemoryUserRepo();
        var queue = new InMemoryQueue();
        ICommandHandler<LogoutUserCommand> handler = new LogoutUserCommandHandler(repo, queue);
        var cmd = new LogoutUserCommand("u1");
        await handler.Handle(cmd, CancellationToken.None);
        Assert.Single(repo.Events);
        Assert.Equal("user-logged-out", repo.Events[0].Type);
    }
}

class InMemoryQueue : IEventQueue
{
    public List<IEvent> Events { get; } = new();
    public Task Add(IEvent ev, CancellationToken ct)
    {
        Events.Add(ev);
        return Task.CompletedTask;
    }
}

class InMemoryTaskRepo : ITaskEventRepository
{
    public List<IEvent> Events { get; } = new();
    public Task Add(IEvent ev, CancellationToken ct)
    {
        Events.Add(ev);
        return Task.CompletedTask;
    }
    public Task<IReadOnlyList<IEvent>> Get(string taskId, CancellationToken ct)
    {
        return Task.FromResult<IReadOnlyList<IEvent>>(Events.Where(e => e.EntityId == taskId).ToList());
    }
}

class InMemoryUserRepo : IUserEventRepository
{
    public List<IEvent> Events { get; } = new();
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
