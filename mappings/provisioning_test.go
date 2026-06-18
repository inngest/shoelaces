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

package mappings

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParamsWithProvisioningProjectsStructuredValuesBeforeDefaults(t *testing.T) {
	timeout := 9
	utc := false
	params := ParamsWithProvisioning(map[string]interface{}{
		"storage_disk": "/dev/explicit",
	}, nil, ProvisioningConfig{
		Locale: LocaleConfig{
			Language: "en_GB.UTF-8",
			Keyboard: "gb",
		},
		Time: TimeConfig{
			Timezone: "Europe/London",
			UTC:      &utc,
		},
		Storage: StorageConfig{
			Disk:        "/dev/structured",
			VolumeGroup: "vgstructured",
		},
		Boot: BootConfig{
			Netboot: NetbootConfig{
				KernelArgs: []string{"console=ttyS1"},
			},
			Installed: InstalledBootConfig{
				TimeoutSeconds: &timeout,
				KernelArgs:     []string{"panic=30"},
			},
		},
		Repos: ReposConfig{
			OSMirror: "https://mirror.example/os",
			Release:  "trixie",
		},
		Installer: InstallerConfig{
			ConfigTemplate: "preseed/custom",
			ConfigParams: map[string]any{
				"encrypt_home": true,
				"token":        "abc",
			},
		},
	})

	assert.Equal(t, "/dev/explicit", params["storage_disk"])
	assert.Equal(t, "/dev/explicit", params["storage_wipe_disks"])
	assert.Equal(t, "vgstructured", params["vg_name"])
	assert.Equal(t, "en_GB.UTF-8", params["locale_language"])
	assert.Equal(t, "gb", params["locale_keyboard"])
	assert.Equal(t, "Europe/London", params["time_timezone"])
	assert.Equal(t, "false", params["time_utc"])
	assert.Equal(t, "", params["kickstart_utc_flag"])
	assert.Equal(t, "console=ttyS1", params["linuxargs"])
	assert.Equal(t, "panic=30", params["boot_installed_args"])
	assert.Equal(t, "9", params["boot_timeout_seconds"])
	assert.Equal(t, "https://mirror.example/os", params["repo_debian_mirror"])
	assert.Equal(t, "trixie", params["release"])
	assert.Equal(t, "preseed/custom", params["debian_installer_config_template"])
	assert.Equal(t, true, params["encrypt_home"])
	installerQuery, err := url.ParseQuery(params["installer_config_query"].(string))
	require.NoError(t, err)
	assert.Equal(t, "true", installerQuery.Get("encrypt_home"))
	assert.Equal(t, "abc", installerQuery.Get("token"))
	assert.NotEmpty(t, installerQuery.Get("provisioning"))
	assert.Equal(t, "&"+params["installer_config_query"].(string), params["installer_config_query_suffix"])
	assert.Equal(t, "?"+params["installer_config_query"].(string), params["installer_config_query_question"])
	assert.Equal(t, "auto", params["iface"])
	assert.Equal(t, "", params["installerExtra"])
}
