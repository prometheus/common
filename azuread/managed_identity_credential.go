package azuread

import (
	"errors"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// Returns new Managed Identity token credential
func NewManagedIdentityTokenCredential(managedIdentityClientId string) (azcore.TokenCredential, error) {
	if len(managedIdentityClientId) > 0 {
		clientId := azidentity.ClientID(managedIdentityClientId)
		opts := &azidentity.ManagedIdentityCredentialOptions{ID: clientId}
		return azidentity.NewManagedIdentityCredential(opts)
	} else {
		return nil, errors.New("The Managed Identity Client ID can not be empty")
	}
}
