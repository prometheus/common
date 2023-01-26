package azuread

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

const (
	dummyAudience   = "dummyAudience"
	dummyClientId   = "00000000-0000-0000-0000-000000000000"
	testTokenString = "testTokenString"
)

var testTokenExpiry = time.Now().Add(10 * time.Second)

type TokenProviderTestSuite struct {
	suite.Suite
	mockCredential *MockCredential
}

func (s *TokenProviderTestSuite) BeforeTest(suiteName, testName string) {
	s.mockCredential = new(MockCredential)
}

func TestTokenProvider(t *testing.T) {
	suite.Run(t, new(TokenProviderTestSuite))
}

func (s *TokenProviderTestSuite) TestNewTokenProvider_NilAudience_Fail() {
	azureAdConfig := &AzureAdConfig{
		Cloud:         "PublicAzure",
		AzureClientId: dummyClientId,
	}

	actualTokenProvider, actualErr := NewTokenProvider(context.Background(), azureAdConfig, s.mockCredential)

	//assert
	s.Assert().Nil(actualTokenProvider)
	s.Assert().NotNil(actualErr)
	s.Assert().Equal("Cloud is not specified or is incorrect: "+azureAdConfig.Cloud, actualErr.Error())
}

func (s *TokenProviderTestSuite) TestNewTokenProvider_Success() {
	azureAdConfig := &AzureAdConfig{
		Cloud:         "AzurePublic",
		AzureClientId: dummyClientId,
	}
	s.mockCredential.On("GetToken", mock.Anything, mock.Anything).Return(getToken(), nil)

	actualTokenProvider, actualErr := NewTokenProvider(context.Background(), azureAdConfig, s.mockCredential)

	//assert
	s.Assert().NotNil(actualTokenProvider)
	s.Assert().Nil(actualErr)
	s.Assert().NotNil(actualTokenProvider.GetAccessToken())
}

func (s *TokenProviderTestSuite) TestPeriodicTokenRefresh_Success() {
	// setup
	azureAdConfig := &AzureAdConfig{
		Cloud:         "AzurePublic",
		AzureClientId: dummyClientId,
	}
	testToken := &azcore.AccessToken{
		Token:     testTokenString,
		ExpiresOn: testTokenExpiry,
	}

	s.mockCredential.On("GetToken", mock.Anything, mock.Anything).Return(*testToken, nil).Once().
		On("GetToken", mock.Anything, mock.Anything).Return(getToken(), nil)

	actualTokenProvider, actualErr := NewTokenProvider(context.Background(), azureAdConfig, s.mockCredential)

	// assert
	s.Assert().NotNil(actualTokenProvider)
	s.Assert().Nil(actualErr)
	s.Assert().NotNil(actualTokenProvider.GetAccessToken())

	// Token set to refresh at half of the expiry time. The test tokens are set to expiry in 10s.
	// Hence, the 6 seconds wait to check if the token is refreshed.
	time.Sleep(6 * time.Second)

	s.mockCredential.AssertNumberOfCalls(s.T(), "GetToken", 2)
	s.Assert().NotEqual(actualTokenProvider.GetAccessToken(), testTokenString)
}

func getToken() azcore.AccessToken {
	return azcore.AccessToken{
		Token:     uuid.New().String(),
		ExpiresOn: time.Now().Add(10 * time.Second),
	}
}
