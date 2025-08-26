using DomainService.Domain;
using System.Text.Json;
using Xunit;

public class SerializationTests
{
    [Fact]
    public void TaskStatusData_serializes_done_property()
    {
        var el = JsonSerializer.SerializeToElement(new TaskStatusData(false));
        Assert.False(el.GetProperty("done").GetBoolean());
    }

    [Fact]
    public void UserProfileData_serializes_expected_properties()
    {
        var el = JsonSerializer.SerializeToElement(new UserProfileData("n", "e"));
        Assert.Equal("n", el.GetProperty("name").GetString());
        Assert.Equal("e", el.GetProperty("email").GetString());
    }

    [Fact]
    public void UserSettingsData_serializes_expected_properties()
    {
        var el = JsonSerializer.SerializeToElement(new UserSettingsData(3, false));
        Assert.Equal(3, el.GetProperty("tasksPerCategory").GetInt32());
        Assert.False(el.GetProperty("showDoneTasks").GetBoolean());
    }
}
