// Copyright 2021 The Prometheus Authors
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

package assets

import (
	"embed"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:embed testdata
var EmbedFS embed.FS

var testFS = New(EmbedFS)

func TestFS(t *testing.T) {
	cases := []struct {
		name            string
		path            string
		expectedSize    int64
		expectedContent string
	}{
		{
			name:            "uncompressed file",
			path:            "testdata/uncompressed",
			expectedSize:    4,
			expectedContent: "foo\n",
		},
		{
			name:            "compressed file",
			path:            "testdata/compressed",
			expectedSize:    4,
			expectedContent: "foo\n",
		},
		{
			name:            "both, open uncompressed",
			path:            "testdata/both",
			expectedSize:    4,
			expectedContent: "foo\n",
		},
		{
			name:         "both, open compressed",
			path:         "testdata/both.gz",
			expectedSize: 29,
			// we don't check content for a explicitly compressed file
			expectedContent: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f, err := testFS.Open(c.path)
			require.NoError(t, err)

			stat, err := f.Stat()
			require.NoError(t, err)

			size := stat.Size()
			require.Equalf(t, c.expectedSize, size, "size is wrong, expected %d, got %d", c.expectedSize, size)

			if strings.HasSuffix(c.path, ".gz") {
				// don't read the comressed content
				return
			}

			content, err := io.ReadAll(f)
			require.NoError(t, err)
			require.Equalf(t, c.expectedContent, string(content), "content is wrong, expected %s, got %s", c.expectedContent, string(content))
		})
	}
}

func TestFileReadAfterEOF(t *testing.T) {
	t.Run("single read reaches EOF and subsequent reads return 0, io.EOF", func(t *testing.T) {
		f, err := testFS.Open("testdata/compressed")
		require.NoError(t, err)
		defer f.Close()

		buf := make([]byte, 16)
		n, err := f.Read(buf)
		require.ErrorIs(t, err, io.EOF)
		require.Equal(t, "foo\n", string(buf[:n]))

		n, err = f.Read(buf)
		require.ErrorIs(t, err, io.EOF)
		require.Zero(t, n)
	})

	t.Run("chunked read advances offset on final EOF chunk", func(t *testing.T) {
		f, err := testFS.Open("testdata/compressed")
		require.NoError(t, err)
		defer f.Close()

		// Zero-length read before EOF returns (0, nil).
		n, err := f.Read(nil)
		require.NoError(t, err)
		require.Zero(t, n)

		buf := make([]byte, 2)
		n, err = f.Read(buf)
		require.NoError(t, err)
		require.Equal(t, "fo", string(buf[:n]))

		n, err = f.Read(buf)
		require.ErrorIs(t, err, io.EOF)
		require.Equal(t, "o\n", string(buf[:n]))

		n, err = f.Read(buf)
		require.ErrorIs(t, err, io.EOF)
		require.Zero(t, n)

		// Zero-length read after EOF returns (0, io.EOF).
		n, err = f.Read(nil)
		require.ErrorIs(t, err, io.EOF)
		require.Zero(t, n)
	})

	t.Run("empty content returns 0, io.EOF immediately", func(t *testing.T) {
		f := &File{}
		buf := make([]byte, 4)
		n, err := f.Read(buf)
		require.ErrorIs(t, err, io.EOF)
		require.Zero(t, n)
	})
}

func TestOpenCorruptGzip(t *testing.T) {
	f, err := testFS.Open("testdata/corrupt")
	require.Error(t, err)
	require.Nil(t, f)
}
