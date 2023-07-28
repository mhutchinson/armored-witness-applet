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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"

	"github.com/coreos/go-semver/semver"
	"github.com/golang/glog"
	"github.com/transparency-dev/formats/log"
)

type LogClient interface {
	GetLeafAndInclusion(index, treeSize uint64) ([]byte, [][]byte, error)
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
	return f.latestOS.version, f.latestApplet.version, nil
}

func (f *HttpFetcher) GetOS() ([]byte, error) {
	if f.latestOS == nil {
		return []byte{}, errors.New("no latest OS available")
	}
	return f.latestOS.data, nil
}

func (f *HttpFetcher) GetApplet() ([]byte, error) {
	if f.latestApplet == nil {
		return []byte{}, errors.New("no latest applet available")
	}
	return f.latestApplet.data, nil
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
		target, version, data, err := parseLeaf(leaf)
		if err != nil {
			return fmt.Errorf("failed to parse leaf at %d: %v", i, err)
		}
		switch target {
		case "trusted_os":
			if f.latestOS == nil || f.latestOS.version.LessThan(version) {
				f.latestOS = &firmwareRelease{
					logIndex: i,
					version:  version,
					data:     data,
				}
			}
		case "trusted_applet":
			if f.latestApplet == nil || f.latestApplet.version.LessThan(version) {
				f.latestApplet = &firmwareRelease{
					logIndex: i,
					version:  version,
					data:     data,
				}
			}
		default:
			glog.Warningf("unknown build in log: %q", target)
		}
	}
	f.latest = to
	return nil
}

func parseLeaf(leaf []byte) (string, semver.Version, []byte, error) {
	panic("not implemented")
}

type firmwareRelease struct {
	logIndex uint64
	version  semver.Version
	data     []byte
}

type HttpLogClient struct {
	logAddress url.URL
	client     http.Client
}

func (f *HttpLogClient) GetLeafAndInclusion(index, treeSize uint64) ([]byte, [][]byte, error) {
	// TODO(mhutchinson): determine real URL
	leafAddr := f.logAddress.JoinPath(fmt.Sprintf("/TODO/v1/%d", index))
	resp, err := f.client.Get(leafAddr.String())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get firmware at %q: %v", leafAddr, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("got non-OK error code from %q: %d", leafAddr, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read body: %v", err)
	}
	// TODO(mhutchinson): get an inclusion proof
	return body, nil, nil
}
