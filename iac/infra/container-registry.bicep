@description('Azure region to deploy resources into. Defaults to location of target resource group')
param location string = resourceGroup().location

@description('Name of the container registry. Uses a deterministic name based on the resource group name.')
param registryName string = 'petspotracr${toLower(replace(resourceGroup().name, '-', ''))}'

resource acr 'Microsoft.ContainerRegistry/registries@2022-02-01-preview' = {
  name: registryName
  location: location
  sku: {
    name: 'Standard'
  }
  properties: {
    anonymousPullEnabled: true
  }
}

output registryId string = acr.id
