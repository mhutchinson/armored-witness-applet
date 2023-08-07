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
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/transparency-dev/armored-witness-applet/api"
)

func NewLogClient(logAddress url.URL, client http.Client) *HttpLogClient {
	return &HttpLogClient{
		logAddress: logAddress,
		client:     client,
	}
}

type HttpLogClient struct {
	logAddress url.URL
	client     http.Client
}

func (f *HttpLogClient) GetLeafAndInclusion(index, treeSize uint64) ([]byte, [][]byte, error) {
	// TODO(mhutchinson): determine real URL
	location := f.logAddress.JoinPath(fmt.Sprintf("/TODO/v1/getLeaf/%d", index))
	resp, err := f.client.Get(location.String())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get firmware at %q: %v", location, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("got non-OK error code from %q: %d", location, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read body: %v", err)
	}
	// TODO(mhutchinson): get an inclusion proof
	return body, nil, nil
}

func (f *HttpLogClient) GetBinary(release api.FirmwareRelease) ([]byte, error) {
	// TODO(mhutchinson): determine real URL
	location := f.logAddress.JoinPath(fmt.Sprintf("/TODO/v1/getBinary/%x", release.FirmwareDigestSha256))
	resp, err := f.client.Get(location.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get firmware binary at %q: %v", location, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got non-OK error code from %q: %d", location, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %v", err)
	}
	return body, nil
}
