// Copyright The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package secret

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v2"
)

func TestSecretRedacted(t *testing.T) {
	type tmp struct {
		S Secret
	}

	c, err := json.Marshal(tmp{"test"})
	require.NoError(t, err)
	require.JSONEq(t, `{"S":"<secret>"}`, string(c))

	c, err = yaml.Marshal(tmp{"test"})
	require.NoError(t, err)
	require.Equal(t, "s: <secret>\n", string(c))

	c, err = json.Marshal(tmp{})
	require.NoError(t, err)
	require.JSONEq(t, `{"S":""}`, string(c))

	var out tmp
	require.NoError(t, yaml.Unmarshal([]byte("s: test\n"), &out))
	require.Equal(t, Secret("test"), out.S)
}
