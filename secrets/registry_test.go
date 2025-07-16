package secrets

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testProvider struct {
	name string
}

func (tp *testProvider) FetchSecret(ctx context.Context) (string, error) {
	return "test_secret", nil
}

func (tp *testProvider) Name() string {
	return tp.name
}

func (tp *testProvider) Key() string {
	return "test_key"
}

func (tp *testProvider) MarshalYAML() (interface{}, error) {
	return nil, nil
}

func TestProviderRegistry(t *testing.T) {
	t.Run("GetInitialProviders", func(t *testing.T) {
		// Test that providers from init() are registered in the global registry.
		p, err := Providers.Get("inline")
		require.NoError(t, err)
		assert.IsType(t, &InlineProvider{}, p)

		p, err = Providers.Get("file")
		require.NoError(t, err)
		assert.IsType(t, &FileProvider{}, p)
	})

	t.Run("GetUnknownProvider", func(t *testing.T) {
		_, err := Providers.Get("unknown-provider")
		require.Error(t, err)
		assert.Contains(t, err.Error(), `unknown provider type: "unknown-provider"`)
	})

	t.Run("RegisterAndGet", func(t *testing.T) {
		reg := &ProviderRegistry{}
		constructor := func() Provider { return &testProvider{name: "test"} }

		reg.Register(constructor)
		p, err := reg.Get("test")
		require.NoError(t, err)
		assert.IsType(t, &testProvider{}, p)
		assert.Equal(t, "test", p.Name())
	})

	t.Run("RegisterDuplicate", func(t *testing.T) {
		reg := &ProviderRegistry{}
		constructor1 := func() Provider { return &testProvider{name: "duplicate"} }
		constructor2 := func() Provider { return &testProvider{name: "duplicate"} }

		reg.Register(constructor1)
		assert.PanicsWithValue(t, `attempt to register duplicate type: "duplicate"`, func() {
			reg.Register(constructor2)
		})
	})
}
