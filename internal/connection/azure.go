package connection

import (
	"context"
	"log"
	"sync"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
	"github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2019-04-01/storage"
	"github.com/citihub/probr-sdk/utils"
)

// AzureCredentials ...
type AzureCredentials struct {
	SubscriptionID, ClientID, TenantID, ClientSecret string
}

// AzureConnection simplifies the connection with cloud provider
type AzureConnection struct {
	isCloudAvailable error
	ctx              context.Context
	credentials      AzureCredentials
	ResourceGroup    *AzureResourceGroup
	Storage          *AzureStorageAccount
}

// Azure interface defining all azure methods
type Azure interface {
	IsCloudAvailable() error
	GetResourceGroupByName(name string) (resources.Group, error)
	CreateStorageAccount(accountName, accountGroupName string, tags map[string]*string, httpsOnly bool, networkRuleSet *storage.NetworkRuleSet) (storage.Account, error)
}

var instance *AzureConnection
var once sync.Once

// NewAzureConnection provides a singleton instance of AzureConnection. Initializes all internal clients to interact with Azure.
func NewAzureConnection(c context.Context, subscriptionID, tenantID, clientID, clientSecret string) (azConn *AzureConnection) {
	once.Do(func() {
		// Guard clause
		if c == nil {
			instance.isCloudAvailable = utils.ReformatError("Context instance cannot be nil")
			return
		}

		instance = &AzureConnection{
			ctx: c,
			credentials: AzureCredentials{
				SubscriptionID: subscriptionID,
				TenantID:       tenantID,
				ClientID:       clientID,
				ClientSecret:   clientSecret,
			},
		}

		// TODO: Initiatlize Authorizer here since it is shared

		// Create an azure resource group client object via the connection config vars
		var grpErr error
		instance.ResourceGroup, grpErr = NewResourceGroup(c, instance.credentials)
		if grpErr != nil {
			instance.isCloudAvailable = utils.ReformatError("Failed to initialize Azure Resource Group: %v", grpErr)
			return
		}

		// Create an azure resource group client object via the connection config vars
		var saErr error
		instance.Storage, grpErr = NewStorageAccount(c, instance.credentials)
		if saErr != nil {
			instance.isCloudAvailable = utils.ReformatError("Failed to initialize Azure Storage Account: %v", grpErr)
			return
		}

		//TODO: Check availability and set isCloudAvailable var
	})
	return instance
}

// IsCloudAvailable verifies that the connection instantiation did not report a failure
func (az *AzureConnection) IsCloudAvailable() error {
	return az.isCloudAvailable
}

// GetResourceGroupByName returns an existing Resource Group by name
func (az *AzureConnection) GetResourceGroupByName(name string) (resources.Group, error) {
	log.Printf("[DEBUG] getting a Resource Group '%s'", name)
	return az.ResourceGroup.Get(name)
}

// CreateStorageAccount creates a storage account
func (az *AzureConnection) CreateStorageAccount(accountName, accountGroupName string, tags map[string]*string, httpsOnly bool, networkRuleSet *storage.NetworkRuleSet) (storage.Account, error) {
	log.Printf("[DEBUG] creating a Storage Account '%s'", accountName)
	return az.Storage.Create(accountName, accountGroupName, tags, httpsOnly, networkRuleSet)
}
