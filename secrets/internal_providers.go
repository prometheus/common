package secrets

import (
	"context"
	"os"
)

// FileProvider fetches secrets from a file.
type FileProvider struct {
	Path string `yaml:"path" json:"path"`
}

func (fp *FileProvider) FetchSecret(ctx context.Context) (string, error) {
	content, err := os.ReadFile(fp.Path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (fp *FileProvider) Name() string {
	return "file"
}

func (fp *FileProvider) Key() string {
	return fp.Path
}

func (fp *FileProvider) MarshalYAML() (interface{}, error) {
	return map[string]interface{}{
		"path": fp.Path,
	}, nil
}

// InlineProvider reads an config secret.
type InlineProvider struct {
	secret string
}

func (ip *InlineProvider) FetchSecret(ctx context.Context) (string, error) {
	return ip.secret, nil
}

func (ip *InlineProvider) Name() string {
	return "inline"
}

func (ip *InlineProvider) Key() string {
	return ip.secret
}

func (ip *InlineProvider) MarshalYAML() (interface{}, error) {
	return "<secret>", nil
}

func init() {
	Providers.Register(func() Provider { return &InlineProvider{} })
	Providers.Register(func() Provider { return &FileProvider{} })
}
