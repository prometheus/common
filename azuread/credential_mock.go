package azuread

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/stretchr/testify/mock"
)

// Mock azidentity TokenCredential interface
type MockCredential struct {
	mock.Mock
}

func (m *MockCredential) GetToken(ctx context.Context, options policy.TokenRequestOptions) (azcore.AccessToken, error) {
	args := m.MethodCalled("GetToken", ctx, options)
	if args.Get(0) == nil {
		return azcore.AccessToken{}, args.Error(1)
	}

	fmt.Println(args.Get(0).(azcore.AccessToken))
	return args.Get(0).(azcore.AccessToken), nil
}
