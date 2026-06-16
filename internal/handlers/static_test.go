// Copyright 2018 ThousandEyes Inc.
//
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

package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOverlayFileServerServesUpperLayerBeforeLowerLayer(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()

	writeTestFile(t, filepath.Join(lower, "shared.txt"), "from lower")
	writeTestFile(t, filepath.Join(upper, "shared.txt"), "from upper")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/shared.txt", nil)

	OverlayFileServer(upper, lower).ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, strings.HasPrefix(rr.Body.String(), "from upper"), "expected upper-layer content, got %q", rr.Body.String())
}

func TestOverlayFileServerFallsBackToLowerLayer(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()

	writeTestFile(t, filepath.Join(lower, "lower-only.txt"), "from lower")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/lower-only.txt", nil)

	OverlayFileServer(upper, lower).ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, strings.HasPrefix(rr.Body.String(), "from lower"), "expected lower-layer content, got %q", rr.Body.String())
}

func TestOverlayFileServerMergesDirectoryIndex(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()

	writeTestFile(t, filepath.Join(lower, "b.txt"), "lower")
	writeTestFile(t, filepath.Join(upper, "a.txt"), "upper")
	writeTestFile(t, filepath.Join(upper, "b.txt"), "upper wins")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	OverlayFileServer(upper, lower).ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, `<a href="a.txt">a.txt</a>`)
	assert.Equal(t, 1, strings.Count(body, `<a href="b.txt">b.txt</a>`))
	assert.Less(t, strings.Index(body, "a.txt"), strings.Index(body, "b.txt"))
}

func TestOverlayFileServerIncludesLowerLayerDirectoryEntries(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()

	require.NoError(t, os.Mkdir(filepath.Join(lower, "lower-dir"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(upper, "upper-dir"), 0o755))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	OverlayFileServer(upper, lower).ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, `<a href="lower-dir/">lower-dir/</a>`)
	assert.Contains(t, body, `<a href="upper-dir/">upper-dir/</a>`)
}

func TestOverlayFileServerReturnsNotFoundWhenFileIsMissing(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/missing.txt", nil)

	OverlayFileServer(upper, lower).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
