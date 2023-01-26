package azuread

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

func loadAzureAdConfig(filename string) (*AzureAdConfig, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	cfg := AzureAdConfig{}
	if err = yaml.UnmarshalStrict(content, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func testGoodConfig(t *testing.T, filename string) {
	_, err := loadAzureAdConfig(filename)
	if err != nil {
		t.Fatalf("Unexpected error parsing %s: %s", filename, err)
	}
}

func TestGoodAzureAdConfig(t *testing.T) {
	filename := "testdata/azuread_good.yaml"
	testGoodConfig(t, filename)
}

func TestBadClientIdMissingAzureAdConfig(t *testing.T) {
	filename := "testdata/azuread_bad_clientidmissing.yaml"
	_, err := loadAzureAdConfig(filename)
	if err == nil {
		t.Fatalf("Did not receive expected error unmarshaling bad azuread config")
	}
	if !strings.Contains(err.Error(), "must provide a Azure Managed Identity clientId in the Azure AD config") {
		t.Errorf("Received unexpected error from unmarshal of %s: %s", filename, err.Error())
	}
}

func TestBadCloudMissingAzureAdConfig(t *testing.T) {
	filename := "testdata/azuread_bad_cloudmissing.yaml"
	_, err := loadAzureAdConfig(filename)
	if err == nil {
		t.Fatalf("Did not receive expected error unmarshaling bad azuread config")
	}
	if !strings.Contains(err.Error(), "must provide Cloud in the Azure AD config") {
		t.Errorf("Received unexpected error from unmarshal of %s: %s", filename, err.Error())
	}
}

func TestBadInvalidClientIdAzureAdConfig(t *testing.T) {
	filename := "testdata/azuread_bad_invalidclientid.yaml"
	_, err := loadAzureAdConfig(filename)
	if err == nil {
		t.Fatalf("Did not receive expected error unmarshaling bad azuread config")
	}
	if !strings.Contains(err.Error(), "Azure Managed Identity clientId provided is invalid") {
		t.Errorf("Received unexpected error from unmarshal of %s: %s", filename, err.Error())
	}
}
