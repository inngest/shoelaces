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
)

func TestOverlayFileServerServesUpperLayerBeforeLowerLayer(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()

	writeTestFile(t, filepath.Join(lower, "shared.txt"), "from lower")
	writeTestFile(t, filepath.Join(upper, "shared.txt"), "from upper")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/shared.txt", nil)

	OverlayFileServer(upper, lower).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if !strings.HasPrefix(rr.Body.String(), "from upper") {
		t.Fatalf("expected upper-layer content, got %q", rr.Body.String())
	}
}

func TestOverlayFileServerFallsBackToLowerLayer(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()

	writeTestFile(t, filepath.Join(lower, "lower-only.txt"), "from lower")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/lower-only.txt", nil)

	OverlayFileServer(upper, lower).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if !strings.HasPrefix(rr.Body.String(), "from lower") {
		t.Fatalf("expected lower-layer content, got %q", rr.Body.String())
	}
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

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `<a href="a.txt">a.txt</a>`) {
		t.Fatalf("directory index missing upper file: %s", body)
	}
	if strings.Count(body, `<a href="b.txt">b.txt</a>`) != 1 {
		t.Fatalf("directory index should de-duplicate overlaid names: %s", body)
	}
	if strings.Index(body, "a.txt") > strings.Index(body, "b.txt") {
		t.Fatalf("directory index should be sorted: %s", body)
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
