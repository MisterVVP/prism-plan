namespace DomainService.Interfaces
{
    public interface ICommandFactory
    {
        public ICommand Create(string queueMessage);
    }
}
