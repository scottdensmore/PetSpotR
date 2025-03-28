using Dapr.Client;

namespace PetSpotR.Data
{
    public class PetModel
    {
        private readonly DaprClient _daprClient;

        public string Name { get; set; }
        public string Type { get; set; }
        public string Breed { get; set; }
        public string OwnerEmail { get; set; }
        public string ID { get; set; }
        public string State { get; set; }
        public List<string> Images { get; set; }


        // Constructor for DI
        public PetModel(DaprClient daprClient)
        {
            _daprClient = daprClient;

            Name = "";
            Type = "";
            Breed = "";
            OwnerEmail = "";
            ID = Guid.NewGuid().ToString();
            State = "new";
            Images = new();
        }

        public async Task SavePetStateAsync(string storeName)
        {
            try {
                await _daprClient.SaveStateAsync(
                    storeName: storeName,
                    key: ID,
                    value: this
                );
            } catch {
                throw;
            }

            return;
        }

        public async Task PublishLostPetAsync(string pubsubName)
        {
            try {
                await _daprClient.PublishEventAsync(
                    pubsubName: pubsubName,
                    topicName: "lostPet",
                    data: new Dictionary<string, string>
                    {
                        { "petId", ID }
                    }
                );
            } catch {
                throw;
            }

            return;
        }
    }
}
