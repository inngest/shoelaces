// Copyright 2026 Inngest Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package persistence

import "time"

// EventRecord is the persisted, already-redacted form of a Shoelaces event.
type EventRecord struct {
	// ID is assigned by durable backends after insertion.
	ID int64
	// Type is the event type value from the event package.
	Type int
	// OccurredAt records when the event happened.
	OccurredAt time.Time
	// MAC is the normalized host MAC address.
	MAC string
	// IP is the host IP observed by Shoelaces.
	IP string
	// Hostname is the resolved or request-provided host name.
	Hostname string
	// BootType describes how the target was selected.
	BootType string
	// Script is the rendered boot script name.
	Script string
	// Message is the operator-facing event message.
	Message string
	// ParamsJSON contains redacted event params encoded as JSON.
	ParamsJSON []byte
}

// ServerStateRecord is a durable snapshot of an in-progress boot selection.
type ServerStateRecord struct {
	// MAC is the normalized host MAC address and primary lookup key.
	MAC string
	// IP is the host IP observed by Shoelaces.
	IP string
	// Hostname is the resolved or request-provided host name.
	Hostname string
	// Target is either the initial waiting sentinel or the selected target.
	Target string
	// Environment is the selected template override environment.
	Environment string
	// ParamsJSON stores resolved template params as JSON.
	ParamsJSON []byte
	// UsersJSON stores resolved structured users as JSON.
	UsersJSON []byte
	// ProvisioningJSON stores resolved structured provisioning config as JSON.
	ProvisioningJSON []byte
	// AllowedTargetsJSON stores manual target options as JSON.
	AllowedTargetsJSON []byte
	// Retry records how many times the host has polled while waiting.
	Retry int64
	// LastAccess records the last poll/update time for expiry cleanup.
	LastAccess time.Time
}

// BootSessionRecord links a boot script to resolved data used by later config requests.
type BootSessionRecord struct {
	// Ref is an opaque lookup token carried by boot/config URLs.
	Ref string
	// MAC is the normalized host MAC address.
	MAC string
	// IP is the host IP observed by Shoelaces.
	IP string
	// Hostname is the resolved or request-provided host name.
	Hostname string
	// Target is the resolved target name or script.
	Target string
	// Environment is the selected template override environment.
	Environment string
	// ParamsJSON stores resolved template params as JSON.
	ParamsJSON []byte
	// UsersJSON stores resolved structured users as JSON.
	UsersJSON []byte
	// ProvisioningJSON stores resolved structured provisioning config as JSON.
	ProvisioningJSON []byte
	// CreatedAt records when the reference was created.
	CreatedAt time.Time
	// ExpiresAt records when the reference should no longer resolve.
	ExpiresAt time.Time
}
