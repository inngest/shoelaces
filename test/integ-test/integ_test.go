//go:build integration

// Copyright 2026 ThousandEyes Inc.
// Copyright 2026 Inngest Inc.
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

package integtest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"syscall"
	"testing"
	"time"
)

const (
	apiAddr = "localhost:18888"
	apiURL  = "http://" + apiAddr
)

type shoelacesProcess struct {
	testDir    string
	fixtureDir string
	client     *http.Client
	cmd        *exec.Cmd
}

type serverInfo struct {
	IP       string
	Mac      string
	Hostname string
}

type eventInfo struct {
	ID         string         `json:"id"`
	EventType  int            `json:"eventType"`
	OccurredAt string         `json:"occurred_at"`
	Server     serverInfo     `json:"server"`
	BootType   string         `json:"bootType"`
	Script     string         `json:"script"`
	Message    string         `json:"message"`
	Params     map[string]any `json:"params"`
}

func TestShoelacesIntegration(t *testing.T) {
	proc := startShoelaces(t)
	defer proc.stop(t)

	t.Run("startup", func(t *testing.T) {
		proc.assertGETStatus(t, "/", http.StatusOK)
	})

	for _, path := range []string{"/", "/events", "/mappings"} {
		t.Run("response success "+path, func(t *testing.T) {
			proc.assertGETStatus(t, path, http.StatusOK)
		})
	}

	for _, tt := range []struct {
		path    string
		fixture string
	}{
		{path: "/static/", fixture: "static.html"},
		{path: "/configs/static/", fixture: "configs-static-default.txt"},
		{path: "/configs/static/rc.local-bootstrap", fixture: "rc.local-bootstrap"},
		{path: "/start", fixture: "start.txt"},
		{path: "/ipxemenu", fixture: "ipxemenu.txt"},
	} {
		t.Run("fixture "+tt.path, func(t *testing.T) {
			proc.assertGETFixture(t, tt.path, tt.fixture)
		})
	}

	t.Run("servers", func(t *testing.T) {
		var expected []serverInfo
		for macLastOctet := 0x00; macLastOctet < 0x100; macLastOctet += 0x11 {
			octet := fmt.Sprintf("%02x", macLastOctet)
			expected = append(expected, serverInfo{
				IP:       "127.0.0.1",
				Mac:      "ff:ff:ff:ff:ff:" + octet,
				Hostname: "localhost",
			})

			proc.assertGETStatus(t, "/poll/1/ff-ff-ff-ff-ff-"+octet, http.StatusOK)

			var got []serverInfo
			proc.getJSON(t, "/ajax/servers", &got)
			sortServers(got)
			sortServers(expected)
			if !reflect.DeepEqual(expected, got) {
				t.Fatalf("server list mismatch\nexpected: %#v\nactual:   %#v", expected, got)
			}
		}
	})

	t.Run("unknown server flow", func(t *testing.T) {
		pollPath := "/poll/1/06-66-de-ad-be-ef"
		proc.assertGETFixture(t, pollPath, "poll-unknown.txt")

		form := url.Values{
			"target":      {"coreos"},
			"mac":         {"06:66:de:ad:be:ef"},
			"version":     {"666.0"},
			"cloudconfig": {"virtual"},
		}
		proc.postForm(t, "/update/target", form)

		proc.assertGETFixture(t, pollPath, "poll-unknown-set-from-ui.txt")
		proc.assertGETFixture(t, pollPath, "poll-unknown.txt")
	})

	t.Run("events", func(t *testing.T) {
		events := map[string][]eventInfo{}
		proc.getJSON(t, "/ajax/events", &events)

		hostEvents := events["06:66:de:ad:be:ef"]
		if len(hostEvents) != 4 {
			t.Fatalf("expected 4 events for unknown server, got %d: %#v", len(hostEvents), hostEvents)
		}
		first := hostEvents[0]
		if first.ID == "" {
			t.Fatal("event id is empty")
		}
		if _, err := time.Parse(time.RFC3339Nano, first.OccurredAt); err != nil {
			t.Fatalf("event occurred_at %q is not RFC3339: %v", first.OccurredAt, err)
		}
		first.ID = ""
		first.OccurredAt = ""

		expected := eventInfo{
			EventType: 0,
			Server: serverInfo{
				IP:       "127.0.0.1",
				Mac:      "06:66:de:ad:be:ef",
				Hostname: "localhost",
			},
			Message: "Host localhost polled for a script.",
		}
		if !reflect.DeepEqual(expected, first) {
			t.Fatalf("first event mismatch\nexpected: %#v\nactual:   %#v", expected, first)
		}
	})

	for _, tt := range []struct {
		name    string
		params  url.Values
		fixture string
	}{
		{name: "default", fixture: "poll.txt"},
		{name: "k8s1-3 staging", params: url.Values{"host": {"k8s1-3"}}, fixture: "poll-k8s1-3-stg.txt"},
		{name: "k8s1-4 staging", params: url.Values{"host": {"k8s1-4"}}, fixture: "poll-k8s1-4-stg.txt"},
		{name: "k8s1-1", params: url.Values{"host": {"k8s1-1"}}, fixture: "poll-k8s1-1.txt"},
		{name: "k8s1-2", params: url.Values{"host": {"k8s1-2"}}, fixture: "poll-k8s1-2.txt"},
	} {
		t.Run("poll "+tt.name, func(t *testing.T) {
			proc.assertGETFixtureWithQuery(t, "/poll/1/ff-ff-ff-ff-ff-ff", tt.params, tt.fixture)
		})
	}

	for _, tt := range []struct {
		name     string
		script   string
		env      string
		expected []string
	}{
		{name: "coreos no env", script: "coreos.ipxe", expected: []string{"cloudconfig", "version"}},
		{name: "coreos default env", script: "coreos.ipxe", env: "default", expected: []string{"cloudconfig", "version"}},
		{name: "coreos production env", script: "coreos.ipxe", env: "production", expected: []string{"cloudconfig", "hostname", "version"}},
	} {
		t.Run("template variables "+tt.name, func(t *testing.T) {
			var got []string
			proc.getJSONWithQuery(t, "/ajax/script/params", url.Values{
				"script":      {tt.script},
				"environment": {tt.env},
			}, &got)
			sort.Strings(got)
			sort.Strings(tt.expected)
			if !reflect.DeepEqual(tt.expected, got) {
				t.Fatalf("template variables mismatch\nexpected: %#v\nactual:   %#v", tt.expected, got)
			}
		})
	}
}

func TestShoelacesStartsWithEmbeddedProvisioningDefaults(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "mappings.yaml"), []byte(`
targets:
  debian12:
    script: debian.ipxe
    params:
      encrypt_home: false
    boot:
      netboot:
        kernelArgs:
          - console=ttyS1
    installer:
      configTemplate: preseed/debian
      configParams:
        site: integ
    repos:
      osMirror: https://deb.example/debian
      release: bookworm
networkMaps:
  - network: 127.0.0.1/32
    defaultTarget: debian12
    targets:
      - debian12
`), 0o644); err != nil {
		t.Fatalf("write mappings: %v", err)
	}

	proc := startShoelacesWithDataDir(t, dataDir)
	defer proc.stop(t)

	proc.assertGETContains(t, "/poll/1/06-66-de-ad-be-ef", nil, []string{
		"Debian bookworm netboot",
		"set mirror https://deb.example/debian/dists/bookworm/",
		"preseed/url=http://localhost:18888/configs/preseed/debian?",
		"encrypt_home=false",
		"site=integ",
		"provisioning=",
		"console=ttyS1",
	})
	proc.assertGETContains(t, "/configs/preseed/debian", url.Values{"encrypt_home": {"false"}}, []string{
		"d-i user-setup/encrypt-home boolean false",
		"d-i finish-install/reboot_in_progress note",
	})
	proc.assertGETContains(t, "/configs/static/provisioning-default.txt", nil, []string{
		"generic embedded provisioning static asset",
	})

	for _, tt := range []struct {
		script   string
		expected []string
	}{
		{script: "debian.ipxe", expected: []string{"encrypt_home"}},
		{script: "preseed/debian", expected: []string{"encrypt_home"}},
	} {
		t.Run("embedded template variables "+tt.script, func(t *testing.T) {
			var got []string
			proc.getJSONWithQuery(t, "/ajax/script/params", url.Values{
				"script": {tt.script},
			}, &got)

			for _, expected := range tt.expected {
				assertStringInSlice(t, got, expected)
			}
		})
	}
}

func TestShoelacesPersistsEventsAcrossRestarts(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "mappings.yaml"), []byte(`
targets:
  debian12:
    script: debian.ipxe
    params:
      encrypt_home: false
      release: bookworm
networkMaps:
  - network: 127.0.0.1/32
    defaultTarget: debian12
    targets:
      - debian12
`), 0o644); err != nil {
		t.Fatalf("write mappings: %v", err)
	}

	const mac = "06:66:de:ad:be:ef"
	proc := startShoelacesWithDataDirAndPersistence(t, dataDir, "sqlite")
	proc.assertGETStatus(t, "/poll/1/06-66-de-ad-be-ef", http.StatusOK)
	var firstEvents map[string][]eventInfo
	proc.getJSON(t, "/ajax/events", &firstEvents)
	if len(firstEvents[mac]) != 1 {
		proc.stop(t)
		t.Fatalf("expected one event before restart, got %#v", firstEvents[mac])
	}
	firstEventID := firstEvents[mac][0].ID
	proc.stop(t)

	proc = startShoelacesWithDataDirAndPersistence(t, dataDir, "sqlite")
	defer proc.stop(t)
	var restartedEvents map[string][]eventInfo
	proc.getJSON(t, "/ajax/events", &restartedEvents)
	if len(restartedEvents[mac]) != 1 {
		t.Fatalf("expected one event after restart, got %#v", restartedEvents[mac])
	}
	if restartedEvents[mac][0].ID != firstEventID {
		t.Fatalf("event id changed across restart: before=%q after=%q", firstEventID, restartedEvents[mac][0].ID)
	}
	if restartedEvents[mac][0].EventType != 2 {
		t.Fatalf("expected persisted HostBoot event, got %#v", restartedEvents[mac][0])
	}
}

func startShoelaces(t *testing.T) *shoelacesProcess {
	t.Helper()
	return startShoelacesWithDataDir(t, "integ-test-configs")
}

func startShoelacesWithDataDir(t *testing.T, dataDir string) *shoelacesProcess {
	t.Helper()
	return startShoelacesWithDataDirAndPersistence(t, dataDir, "memory")
}

func startShoelacesWithDataDirAndPersistence(t *testing.T, dataDir string, persistenceBackend string) *shoelacesProcess {
	t.Helper()

	testDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	baseDir, err := filepath.Abs(filepath.Join(testDir, "..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}

	binary := filepath.Join(t.TempDir(), "shoelaces")
	build := exec.Command("go", "build", "-o", binary, "./cmd/shoelaces")
	build.Dir = baseDir
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build shoelaces binary: %v\n%s", err, output)
	}

	configPath := filepath.Join(t.TempDir(), "shoelaces.toml")
	config := fmt.Sprintf(`[network]
bindAddr = "%s"

[data]
dir = "%s"

[template]
extension = ".slc"

[mappings]
file = "mappings.yaml"

[log]
level = "debug"

[persistence]
backend = "%s"
`, apiAddr, dataDir, persistenceBackend)
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write integration config: %v", err)
	}

	cmd := exec.Command(binary, "-config", configPath)
	cmd.Dir = testDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var stderr bytes.Buffer
	cmd.Stdout = io.Discard
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start shoelaces: %v\n%s", err, stderr.String())
	}

	proc := &shoelacesProcess{
		testDir:    testDir,
		fixtureDir: filepath.Join(testDir, "expected-results"),
		client:     &http.Client{Timeout: 5 * time.Second},
		cmd:        cmd,
	}
	proc.waitForStartup(t, &stderr)
	return proc
}

func (p *shoelacesProcess) stop(t *testing.T) {
	t.Helper()

	if p.cmd.Process != nil {
		_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGTERM)
	}
	done := make(chan error, 1)
	go func() {
		done <- p.cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil && !isExpectedSignal(err) {
			t.Fatalf("wait for shoelaces shutdown: %v", err)
		}
	case <-time.After(5 * time.Second):
		if p.cmd.Process != nil {
			_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
		}
		err := <-done
		if err != nil && !isExpectedSignal(err) {
			t.Fatalf("wait for shoelaces shutdown after SIGKILL: %v", err)
		}
	}
}

func isExpectedSignal(err error) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	return ok && status.Signaled()
}

func (p *shoelacesProcess) waitForStartup(t *testing.T, stderr *bytes.Buffer) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := p.client.Get(apiURL + "/")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		if p.cmd.ProcessState != nil && p.cmd.ProcessState.Exited() {
			t.Fatalf("shoelaces exited before startup\n%s", stderr.String())
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("shoelaces did not start before timeout\n%s", stderr.String())
}

func (p *shoelacesProcess) assertGETStatus(t *testing.T, path string, status int) {
	t.Helper()

	resp := p.get(t, path, nil)
	defer resp.Body.Close()
	if resp.StatusCode != status {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET %s status = %d, want %d\n%s", path, resp.StatusCode, status, body)
	}
}

func (p *shoelacesProcess) assertGETFixture(t *testing.T, path string, fixture string) {
	t.Helper()
	p.assertGETFixtureWithQuery(t, path, nil, fixture)
}

func (p *shoelacesProcess) assertGETFixtureWithQuery(t *testing.T, path string, query url.Values, fixture string) {
	t.Helper()

	resp := p.get(t, path, query)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET %s status = %d, want 200\n%s", path, resp.StatusCode, body)
	}
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read GET %s response: %v", path, err)
	}
	expected, err := os.ReadFile(filepath.Join(p.fixtureDir, fixture))
	if err != nil {
		t.Fatalf("read fixture %s: %v", fixture, err)
	}
	if string(expected) != string(got) {
		t.Fatalf("GET %s response mismatch with %s\nexpected:\n%s\nactual:\n%s", path, fixture, expected, got)
	}
}

func (p *shoelacesProcess) assertGETContains(t *testing.T, path string, query url.Values, expected []string) {
	t.Helper()

	resp := p.get(t, path, query)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET %s status = %d, want 200\n%s", path, resp.StatusCode, body)
	}
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read GET %s response: %v", path, err)
	}
	for _, want := range expected {
		if !bytes.Contains(got, []byte(want)) {
			t.Fatalf("GET %s response missing %q\nresponse:\n%s", path, want, got)
		}
	}
}

func (p *shoelacesProcess) getJSON(t *testing.T, path string, out any) {
	t.Helper()
	p.getJSONWithQuery(t, path, nil, out)
}

func (p *shoelacesProcess) getJSONWithQuery(t *testing.T, path string, query url.Values, out any) {
	t.Helper()

	resp := p.get(t, path, query)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET %s status = %d, want 200\n%s", path, resp.StatusCode, body)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("decode GET %s JSON: %v", path, err)
	}
}

func assertStringInSlice(t *testing.T, got []string, expected string) {
	t.Helper()
	for _, item := range got {
		if item == expected {
			return
		}
	}
	t.Fatalf("expected %q in %#v", expected, got)
}

func (p *shoelacesProcess) get(t *testing.T, path string, query url.Values) *http.Response {
	t.Helper()

	requestURL := apiURL + path
	if len(query) > 0 {
		requestURL += "?" + query.Encode()
	}
	resp, err := p.client.Get(requestURL)
	if err != nil {
		t.Fatalf("GET %s: %v", requestURL, err)
	}
	return resp
}

func (p *shoelacesProcess) postForm(t *testing.T, path string, form url.Values) {
	t.Helper()

	resp, err := p.client.PostForm(apiURL+path, form)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST %s status = %d, want 2xx\n%s", path, resp.StatusCode, body)
	}
}

func sortServers(servers []serverInfo) {
	sort.Slice(servers, func(i, j int) bool {
		return servers[i].Mac < servers[j].Mac
	})
}
