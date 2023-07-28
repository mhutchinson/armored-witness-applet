// Copyright 2023 The Armored Witness Applet authors. All Rights Reserved.
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

// Package update provides functionality for fetching updates, verifying
// them, and installing them onto the armory device.
package update

import (
	"context"
	"fmt"

	"github.com/coreos/go-semver/semver"
	"github.com/golang/glog"
)

type Local interface {
	GetInstalledVersions() (os, applet semver.Version, err error)
	InstallOS([]byte) error
	InstallApplet([]byte) error
}

type Remote interface {
	GetLatestVersions() (os, applet semver.Version, err error)
	GetOS() ([]byte, error)
	GetApplet() ([]byte, error)
}

type FirmwareVerifier interface {
	VerifyAndUnpack([]byte) ([]byte, error)
}

func NewUpdater(local Local, remote Remote, verifier FirmwareVerifier) (*Updater, error) {
	osVer, appVer, err := local.GetInstalledVersions()
	if err != nil {
		return nil, fmt.Errorf("failed to determine installed versions: %v", err)
	}
	return &Updater{
		local:    local,
		remote:   remote,
		verifier: verifier,
		osVer:    osVer,
		appVer:   appVer,
	}, nil
}

type Updater struct {
	local         Local
	remote        Remote
	verifier      FirmwareVerifier
	osVer, appVer semver.Version
}

func (u Updater) Update(ctx context.Context) (bool, error) {
	reboot := false
	osVer, appVer, err := u.remote.GetLatestVersions()
	if err != nil {
		return reboot, fmt.Errorf("failed to get latest versions: %v", err)
	}
	if u.osVer.LessThan(osVer) {
		glog.Infof("Upgrading OS from %q to %q", u.osVer, osVer)
		bundle, err := u.remote.GetOS()
		if err != nil {
			return reboot, fmt.Errorf("failed to fetch OS firmware: %v", err)
		}
		firmware, err := u.verifier.VerifyAndUnpack(bundle)
		if err != nil {
			return reboot, fmt.Errorf("verification of OS firmware bundle failed: %v", err)
		}
		if err := u.local.InstallOS(firmware); err != nil {
			return reboot, fmt.Errorf("failed to install OS firmware: %v", err)
		}
		reboot = true
	}
	if u.appVer.LessThan(appVer) {
		glog.Infof("Upgrading applet from %q to %q", u.osVer, osVer)
		bundle, err := u.remote.GetApplet()
		if err != nil {
			return reboot, fmt.Errorf("failed to fetch applet firmware: %v", err)
		}
		firmware, err := u.verifier.VerifyAndUnpack(bundle)
		if err != nil {
			return reboot, fmt.Errorf("verification of applet firmware bundle failed: %v", err)
		}
		if err := u.local.InstallApplet(firmware); err != nil {
			return reboot, fmt.Errorf("failed to install applet firmware: %v", err)
		}
		reboot = true
	}
	return reboot, nil
}
