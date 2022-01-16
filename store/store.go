// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package store contains interfaces for storing data needed for WhatsApp multidevice.
package store

import (
	"time"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/util/keys"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type IdentityStore interface {
	PutIdentity(address string, key [32]byte) error
	DeleteAllIdentities(phone string) error
	DeleteIdentity(address string) error
	IsTrustedIdentity(address string, key [32]byte) (bool, error)
}

type SessionStore interface {
	GetSession(address string) ([]byte, error)
	HasSession(address string) (bool, error)
	PutSession(address string, session []byte) error
	DeleteAllSessions(phone string) error
	DeleteSession(address string) error
}

type PreKeyStore interface {
	GetOrGenPreKeys(count uint32) ([]*keys.PreKey, error)
	GenOnePreKey() (*keys.PreKey, error)
	GetPreKey(id uint32) (*keys.PreKey, error)
	RemovePreKey(id uint32) error
	MarkPreKeysAsUploaded(upToID uint32) error
	UploadedPreKeyCount() (int, error)
}

type SenderKeyStore interface {
	PutSenderKey(group, user string, session []byte) error
	GetSenderKey(group, user string) ([]byte, error)
}

type AppStateSyncKey struct {
	Data        []byte `json:"data"`
	Fingerprint []byte `json:"fingerprint"`
	Timestamp   int64  `json:"timestamp"`
}

type AppStateSyncKeyStore interface {
	PutAppStateSyncKey(id []byte, key AppStateSyncKey) error
	GetAppStateSyncKey(id []byte) (*AppStateSyncKey, error)
}

type AppStateMutationMAC struct {
	IndexMAC []byte `json:"indexMac"`
	ValueMAC []byte `json:"valueMac"`
}

type AppStateStore interface {
	PutAppStateVersion(name string, version uint64, hash [128]byte) error
	GetAppStateVersion(name string) (uint64, [128]byte, error)
	DeleteAppStateVersion(name string) error

	PutAppStateMutationMACs(name string, version uint64, mutations []AppStateMutationMAC) error
	DeleteAppStateMutationMACs(name string, indexMACs [][]byte) error
	GetAppStateMutationMAC(name string, indexMAC []byte) (valueMAC []byte, err error)
}

type ContactStore interface {
	PutPushName(user types.JID, pushName string) (bool, string, error)
	PutBusinessName(user types.JID, businessName string) error
	PutContactName(user types.JID, fullName, firstName string) error
	GetContact(user types.JID) (types.ContactInfo, error)
	GetAllContacts() (map[types.JID]types.ContactInfo, error)
}

type ChatSettingsStore interface {
	PutMutedUntil(chat types.JID, mutedUntil time.Time) error
	PutPinned(chat types.JID, pinned bool) error
	PutArchived(chat types.JID, archived bool) error
	GetChatSettings(chat types.JID) (types.LocalChatSettings, error)
}

type DeviceContainer interface {
	PutDevice(store *Device) error
	DeleteDevice(store *Device) error
}

type Device struct {
	Log waLog.Logger `json:"-"`

	NoiseKey       *keys.KeyPair `json:"noiseKey"`
	IdentityKey    *keys.KeyPair `json:"identityKey"`
	SignedPreKey   *keys.PreKey  `json:"signedPreKey"`
	RegistrationID uint32        `json:"registrationId"`
	AdvSecretKey   []byte        `json:"advSecretKey"`

	ID           *types.JID                       `json:"id"`
	Account      *waProto.ADVSignedDeviceIdentity `json:"account"`
	Platform     string                           `json:"platform"`
	BusinessName string                           `json:"businessName"`
	PushName     string                           `json:"pushName"`

	Initialized  bool `json:"-"`
	Identities   IdentityStore
	Sessions     SessionStore
	PreKeys      PreKeyStore
	SenderKeys   SenderKeyStore
	AppStateKeys AppStateSyncKeyStore
	AppState     AppStateStore
	Contacts     ContactStore
	ChatSettings ChatSettingsStore
	Container    DeviceContainer
}

func (device *Device) Save() error {
	return device.Container.PutDevice(device)
}

func (device *Device) Delete() error {
	err := device.Container.DeleteDevice(device)
	if err != nil {
		return err
	}
	device.ID = nil
	return nil
}
