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

import "fmt"

// UserConfig describes one username-keyed account entry from mappings.yaml.
// Pointer booleans preserve the difference between "unset" and an explicit
// false override while merging defaults, target users, and mapping users.
type UserConfig struct {
	// Primary marks the non-root account used by installer flows that require a
	// single regular install user.
	Primary *bool `koanf:"primary"`
	// System marks a system account. The username "root" is treated as system
	// during resolution even when this field is unset.
	System *bool `koanf:"system"`
	// Locked disables password login for this account when true.
	Locked *bool `koanf:"locked"`
	// Absent suppresses an inherited account when true.
	Absent *bool `koanf:"absent"`
	// FullName is the account gecos/display name.
	FullName string `koanf:"fullName"`
	// PasswordCrypted is a crypt(3)-style password hash or { env: ENV_VAR }.
	PasswordCrypted any `koanf:"passwordCrypted"`
	// SSHAuthorizedKeys contains raw public keys or { env: ENV_VAR } entries.
	SSHAuthorizedKeys []any `koanf:"sshAuthorizedKeys"`
	// Groups contains supplementary group names.
	Groups []string `koanf:"groups"`
	// Shell is the login shell path.
	Shell string `koanf:"shell"`
	// Sudo is an optional sudoers rule for this account.
	Sudo string `koanf:"sudo"`
}

// ResolvedUser is a normalized account entry ready for renderer phases.
type ResolvedUser struct {
	// Name is the username key from mappings.yaml.
	Name string
	// Primary marks the non-root install user.
	Primary bool
	// System marks system accounts. Root is always resolved as a system account.
	System bool
	// Locked disables password login for this account.
	Locked bool
	// FullName is the account gecos/display name.
	FullName string
	// PasswordCrypted is a resolved crypt(3)-style password hash.
	PasswordCrypted string
	// SSHAuthorizedKeys contains resolved SSH public keys.
	SSHAuthorizedKeys []string
	// Groups contains supplementary group names.
	Groups []string
	// Shell is the login shell path.
	Shell string
	// Sudo is an optional sudoers rule for this account.
	Sudo string
}

func mergeUserConfigMap(dst map[string]UserConfig, src map[string]UserConfig) {
	for name, user := range src {
		dst[name] = mergeUserConfig(dst[name], user)
	}
}

func mergeUserConfig(base UserConfig, override UserConfig) UserConfig {
	if override.Primary != nil {
		base.Primary = copyBoolPtr(override.Primary)
	}
	if override.System != nil {
		base.System = copyBoolPtr(override.System)
	}
	if override.Locked != nil {
		base.Locked = copyBoolPtr(override.Locked)
	}
	if override.Absent != nil {
		base.Absent = copyBoolPtr(override.Absent)
	}
	if override.FullName != "" {
		base.FullName = override.FullName
	}
	if override.PasswordCrypted != nil {
		base.PasswordCrypted = override.PasswordCrypted
	}
	if override.SSHAuthorizedKeys != nil {
		base.SSHAuthorizedKeys = append([]any(nil), override.SSHAuthorizedKeys...)
	}
	if override.Groups != nil {
		base.Groups = append([]string(nil), override.Groups...)
	}
	if override.Shell != "" {
		base.Shell = override.Shell
	}
	if override.Sudo != "" {
		base.Sudo = override.Sudo
	}
	return base
}

func resolveUser(name string, user UserConfig, lookup EnvLookup) (ResolvedUser, error) {
	resolved := ResolvedUser{
		Name:     name,
		Primary:  boolValue(user.Primary),
		System:   boolValue(user.System) || name == "root",
		Locked:   boolValue(user.Locked),
		FullName: user.FullName,
		Groups:   append([]string(nil), user.Groups...),
		Shell:    user.Shell,
		Sudo:     user.Sudo,
	}

	if user.PasswordCrypted != nil {
		passwordCrypted, err := resolveStringValue(fmt.Sprintf(`user %q passwordCrypted`, name), user.PasswordCrypted, lookup)
		if err != nil {
			return ResolvedUser{}, err
		}
		resolved.PasswordCrypted = passwordCrypted
	}

	for i, key := range user.SSHAuthorizedKeys {
		authorizedKey, err := resolveStringValue(fmt.Sprintf(`user %q sshAuthorizedKeys[%d]`, name, i), key, lookup)
		if err != nil {
			return ResolvedUser{}, err
		}
		resolved.SSHAuthorizedKeys = append(resolved.SSHAuthorizedKeys, authorizedKey)
	}

	return resolved, nil
}

func resolveStringValue(field string, value any, lookup EnvLookup) (string, error) {
	resolved, err := resolveParamValue(field, value, lookup)
	if err != nil {
		return "", err
	}
	asString, ok := resolved.(string)
	if !ok {
		return "", fmt.Errorf("%s must resolve to a string", field)
	}
	return asString, nil
}

func copyUserConfigMap(users map[string]UserConfig) map[string]UserConfig {
	if users == nil {
		return nil
	}
	copied := make(map[string]UserConfig, len(users))
	for name, user := range users {
		copied[name] = copyUserConfig(user)
	}
	return copied
}

func copyUserConfig(user UserConfig) UserConfig {
	user.Primary = copyBoolPtr(user.Primary)
	user.System = copyBoolPtr(user.System)
	user.Locked = copyBoolPtr(user.Locked)
	user.Absent = copyBoolPtr(user.Absent)
	user.SSHAuthorizedKeys = append([]any(nil), user.SSHAuthorizedKeys...)
	user.Groups = append([]string(nil), user.Groups...)
	return user
}

func boolValue(value *bool) bool {
	return value != nil && *value
}

func copyBoolPtr(value *bool) *bool {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}
