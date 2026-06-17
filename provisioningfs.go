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

package shoelaces

import (
	"embed"
	"io/fs"
)

//go:embed configs/data-dir/cloud-config configs/data-dir/ipxe configs/data-dir/kickstart configs/data-dir/preseed configs/data-dir/static/provisioning-default.txt
var embeddedProvisioningDefaults embed.FS

// ProvisioningDefaultsFS returns the embedded generic provisioning defaults.
//
// The filesystem is rooted like data-dir and intentionally excludes runtime
// site policy and site-only examples such as mappings.yaml, firstboot scripts,
// SSH key material, and other static bootstrap artifacts.
func ProvisioningDefaultsFS() fs.FS {
	return mustSub(embeddedProvisioningDefaults, "configs/data-dir")
}
