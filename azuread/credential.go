package azuread

import (
	"errors"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

// NewTokenCredential return TokenCredential of different kinds like Azure Managed Identity and Azure AD application.
func NewTokenCredential(cfg *AzureAdConfig) (azcore.TokenCredential, error) {
	var cred azcore.TokenCredential
	var err error
	if len(cfg.AzureClientId) > 0 {
		cred, err = NewManagedIdentityTokenCredential(cfg.AzureClientId)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("Azure Client ID is invalid")
	}

	return cred, nil
}
