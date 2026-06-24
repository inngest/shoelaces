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
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var genericProvisioningDefaultCandidates = []string{
	"cloud-config/cloud-config-release.slc",
	"generated/debian/luks-tpm-setup.sh.slc",
	"generated/plain/firstboot.defaults.slc",
	"generated/plain/luks-tpm.passphrase.slc",
	"generated/static/firstboot.service.slc",
	"generated/static/firstboot.sh.slc",
	"ipxe/centos.ipxe.slc",
	"ipxe/coreos.ipxe.slc",
	"ipxe/debian.ipxe.slc",
	"ipxe/linux.cfg.slc",
	"ipxe/storage.ipxe.slc",
	"ipxe/ubuntu-minimal.ipxe.slc",
	"kickstart/centos.ks.slc",
	"preseed/common.preseed.slc",
	"preseed/debian.preseed.slc",
	"preseed/storage.preseed.slc",
	"preseed/ubuntu-minimal.preseed.slc",
	"provisioning/extra.slc",
	"static/plain-luks-autopartition-crypto.sh",
	"static/provisioning-default.txt",
}

var siteOnlyProvisioningFiles = []string{
	"mappings.yaml",
	"plain/firstboot.defaults.slc",
	"static/authorized_keys",
	"static/firstboot.service",
	"static/firstboot.sh",
	"static/id_ansible",
	"static/test-script",
}

var siteOnlyProvisioningMarkers = []string{
	"git@example.com:infra/provisioning.git",
	"/configs/static/firstboot",
	"/static/authorized_keys",
	"/static/id_ansible",
	"ANSIBLE_REPO_URL",
	"ANSIBLE_PLAYBOOK",
	"id_ansible",
	"rootpw root",
	"ssh-rsa fake-key",
	"$6$6C2rNs/iVblJ.PbR$",
}

func TestGenericProvisioningDefaultCandidatesExcludeSiteOnlyFiles(t *testing.T) {
	candidates := make(map[string]struct{}, len(genericProvisioningDefaultCandidates))
	for _, candidate := range genericProvisioningDefaultCandidates {
		candidates[candidate] = struct{}{}
	}
	for _, excluded := range siteOnlyProvisioningFiles {
		if _, ok := candidates[excluded]; ok {
			t.Fatalf("site-only provisioning file %q must not be an embedded default candidate", excluded)
		}
	}
}

func TestGenericProvisioningDefaultCandidatesDoNotContainSiteOnlyMarkers(t *testing.T) {
	for _, candidate := range genericProvisioningDefaultCandidates {
		t.Run(candidate, func(t *testing.T) {
			content := readProvisioningCandidate(t, candidate)
			for _, marker := range siteOnlyProvisioningMarkers {
				if strings.Contains(content, marker) {
					t.Fatalf("generic provisioning default candidate %q contains site-only marker %q", candidate, marker)
				}
			}
		})
	}
}

func TestEmbeddedProvisioningDefaultFilesExist(t *testing.T) {
	defaults := ProvisioningDefaultsFS()
	for _, candidate := range genericProvisioningDefaultCandidates {
		t.Run(candidate, func(t *testing.T) {
			info, err := fs.Stat(defaults, candidate)
			if err != nil {
				t.Fatalf("expected %q in embedded provisioning defaults: %v", candidate, err)
			}
			if info.IsDir() {
				t.Fatalf("expected %q to be a file", candidate)
			}
			if info.Size() == 0 {
				t.Fatalf("expected %q to be non-empty", candidate)
			}
		})
	}
}

func TestEmbeddedProvisioningDefaultsExcludeSiteOnlyFiles(t *testing.T) {
	defaults := ProvisioningDefaultsFS()
	for _, excluded := range siteOnlyProvisioningFiles {
		t.Run(excluded, func(t *testing.T) {
			if _, err := fs.Stat(defaults, excluded); err == nil {
				t.Fatalf("site-only provisioning file %q must not be embedded", excluded)
			}
		})
	}
}

func readProvisioningCandidate(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join("configs", "data-dir", path))
	if err != nil {
		t.Fatalf("read provisioning candidate %q: %v", path, err)
	}
	return string(content)
}
