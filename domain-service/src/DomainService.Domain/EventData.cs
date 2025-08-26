namespace DomainService.Domain;

using System.Text.Json.Serialization;

public sealed record TaskStatusData(
    [property: JsonPropertyName("done")] bool Done);

public sealed record UserProfileData(
    [property: JsonPropertyName("name")] string Name,
    [property: JsonPropertyName("email")] string Email);

public sealed record UserSettingsData(
    [property: JsonPropertyName("tasksPerCategory")] int TasksPerCategory,
    [property: JsonPropertyName("showDoneTasks")] bool ShowDoneTasks);

