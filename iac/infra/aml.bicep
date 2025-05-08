@description('Azure region to deploy resources into. Defaults to location of target resource group')
param location string = resourceGroup().location

@description('Name of the AML workspace. Uses a deterministic name based on the resource group name.')
param workspaceName string = 'petspotr-ml-${toLower(replace(resourceGroup().name, '-', ''))}'

@description('ARM ID of the storage account used by the workspace.')
param storageAccountId string

@description('ARM ID of the Azure Key Vault used by the workspace.')
param keyVaultId string

@description('ARM ID of the Azure Container Registry.')
param containerRegistryId string

resource workspace 'Microsoft.MachineLearningServices/workspaces@2022-10-01' = {
  name: workspaceName
  location: location
  properties: {
    friendlyName: workspaceName
    storageAccount: storageAccountId
    keyVault: keyVaultId
    containerRegistry: containerRegistryId
    applicationInsights: resourceId('Microsoft.Insights/components', 'petspotr-ai-${toLower(replace(resourceGroup().name, '-', ''))}')
    hbiWorkspace: false
    allowPublicAccessWhenBehindVnet: false
  }
  identity: {
    type: 'SystemAssigned'
  }
}

resource applicationInsights 'Microsoft.Insights/components@2020-02-02' = {
  name: 'petspotr-ai-${toLower(replace(resourceGroup().name, '-', ''))}'
  location: location
  kind: 'web'
  properties: {
    Application_Type: 'web'
    WorkspaceResourceId: resourceId('Microsoft.OperationalInsights/workspaces', 'petspotr-law-${toLower(replace(resourceGroup().name, '-', ''))}')
  }
}

resource logAnalyticsWorkspace 'Microsoft.OperationalInsights/workspaces@2022-10-01' = {
  name: 'petspotr-law-${toLower(replace(resourceGroup().name, '-', ''))}'
  location: location
  properties: {
    sku: {
      name: 'PerGB2018'
    }
    retentionInDays: 30
    features: {
      enableLogAccessUsingOnlyResourcePermissions: true
    }
  }
}
