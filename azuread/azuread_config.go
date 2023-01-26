package azuread

import (
	"fmt"

	"github.com/google/uuid"
)

// AzureAdConfig is the configuration for getting the accessToken
// for remote write requests to Azure Monitoring Workspace
type AzureAdConfig struct {
	// AzureClientId is the clientId of the managed identity that is being used to authenticate.
	AzureClientId string `yaml:"azure_client_id,omitempty"`

	// Cloud is the Azure cloud in which the service is running. Example: AzurePublic/AzureGovernment/AzureChina
	Cloud string `yaml:"cloud,omitempty"`
}

// Used to validate config values provided
func (c *AzureAdConfig) Validate() error {
	if c.Cloud == "" {
		return fmt.Errorf("must provide Cloud in the Azure AD config")
	}

	if c.AzureClientId == "" {
		return fmt.Errorf("must provide a Azure Managed Identity clientId in the Azure AD config")
	}

	_, err := uuid.Parse(c.AzureClientId)

	if err != nil {
		return fmt.Errorf("Azure Managed Identity clientId provided is invalid")
	}
	return nil
}

// Used to unmarshal Azure Ad config yaml
func (c *AzureAdConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain AzureAdConfig
	*c = AzureAdConfig{}
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}
	return c.Validate()
}
