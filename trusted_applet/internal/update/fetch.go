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

package update

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/coreos/go-semver/semver"
	"github.com/golang/glog"
	"github.com/transparency-dev/armored-witness-applet/api"
	"github.com/transparency-dev/formats/log"
)

type LogClient interface {
	GetLeafAndInclusion(index, treeSize uint64) ([]byte, [][]byte, error)
	GetBinary(release api.FirmwareRelease) ([]byte, error)
}

func NewHttpFetcher(client LogClient) *HttpFetcher {
	// TODO(mhutchinson): This should take a channel that provides witnessed checkpoints for
	// the firmware log. This means that this witness can only be running software that is
	// committed to by the log checkpoints that it signs. It also reduces the number of fetch
	// operations during normal use.
	f := &HttpFetcher{
		client: client,
	}
	return f
}

type HttpFetcher struct {
	client LogClient

	mu           sync.Mutex
	latest       log.Checkpoint
	latestOS     *firmwareRelease
	latestApplet *firmwareRelease
}

func (f *HttpFetcher) GetLatestVersions() (os semver.Version, applet semver.Version, err error) {
	if f.latestOS == nil || f.latestApplet == nil {
		return semver.Version{}, semver.Version{}, errors.New("no versions of OS or applet found in log")
	}
	return f.latestOS.manifest.GitTagName, f.latestApplet.manifest.GitTagName, nil
}

func (f *HttpFetcher) GetOS() ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.latestOS == nil {
		return nil, errors.New("no latest OS available")
	}
	if f.latestOS.binary == nil {
		binary, err := f.client.GetBinary(f.latestOS.manifest)
		if err != nil {
			return nil, fmt.Errorf("failed to get binary: %v", err)
		}
		f.latestOS.binary = binary
	}
	return f.latestOS.binary, nil
}

func (f *HttpFetcher) GetApplet() ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.latestApplet == nil {
		return nil, errors.New("no latest applet available")
	}
	if f.latestApplet.binary == nil {
		binary, err := f.client.GetBinary(f.latestApplet.manifest)
		if err != nil {
			return nil, fmt.Errorf("failed to get binary: %v", err)
		}
		f.latestApplet.binary = binary
	}
	return f.latestApplet.binary, nil
}

// Notify should be called when a new checkpoint is available.
// This triggers a scan of the new entries in the log to find new firmware.
// TODO(mhutchinson): replace this with a channel?
func (f *HttpFetcher) Notify(cp log.Checkpoint) {
	if err := f.scanTo(cp); err != nil {
		glog.Warningf("failed to scan to latest checkpoint: %v", err)
	}
}

func (f *HttpFetcher) scanTo(to log.Checkpoint) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	from := f.latest.Size
	if to.Size <= from {
		return nil
	}
	for i := from; i < to.Size; i++ {
		// TODO(mhutchinson): collect the inclusion proof and use it
		leaf, _, err := f.client.GetLeafAndInclusion(i, to.Size)
		if err != nil {
			return fmt.Errorf("failed to get log leaf %d: %v", i, err)
		}
		manifest, err := parseLeaf(leaf)
		if err != nil {
			return fmt.Errorf("failed to parse leaf at %d: %v", i, err)
		}
		switch manifest.Component {
		case api.ComponentOS:
			if f.latestOS == nil || f.latestOS.manifest.GitTagName.LessThan(manifest.GitTagName) {
				f.latestOS = &firmwareRelease{
					logIndex: i,
					manifest: manifest,
				}
			}
		case api.ComponentApplet:
			if f.latestApplet == nil || f.latestApplet.manifest.GitTagName.LessThan(manifest.GitTagName) {
				f.latestApplet = &firmwareRelease{
					logIndex: i,
					manifest: manifest,
				}
			}
		default:
			glog.Warningf("unknown build in log: %q", manifest.Component)
		}
	}
	f.latest = to
	return nil
}

func parseLeaf(leaf []byte) (api.FirmwareRelease, error) {
	r := api.FirmwareRelease{}
	if err := json.Unmarshal(leaf, &r); err != nil {
		return r, fmt.Errorf("Unmarshal: %v", err)
	}
	return r, nil
}

type firmwareRelease struct {
	logIndex uint64
	manifest api.FirmwareRelease
	binary   []byte
}
