package config

import (
	"bytes"
	"testing"

	"gopkg.in/yaml.v2"
)

func TestSecretMarshalYAML(t *testing.T) {
	var newSecret = Secret("test-secret")
	var expectedResult = []byte("test-secret\n")

	result, err := yaml.Marshal(newSecret)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(result, expectedResult) {
		t.Errorf("The expected value (%#q) differs from the obtained value (%#q)", expectedResult, result)
	}
}
