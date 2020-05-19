// Copyright 2015 CoreOS, Inc.
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

// The vultr provider fetches a remote configuration from the vultr
// user-data metadata service URL.
// https://web.archive.org/web/20190513194756/https://www.vultr.com/metadata/#user

package vultr

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/ignition/config"
	"github.com/coreos/ignition/src/log"
	"github.com/coreos/ignition/src/providers"
	"github.com/coreos/ignition/src/providers/util"
)

const (
	name           = "vultr"
	initialBackoff = 100 * time.Millisecond
	maxBackoff     = 30 * time.Second
	host           = "http://169.254.169.254/"
	dataUrl           = host + "metadata/v1/user-data"
)

func init() {
	providers.Register(creator{})
}

type creator struct{}

func (creator) Name() string {
	return name
}

func (creator) Create(logger log.Logger) providers.Provider {
	return &provider{
		logger:  logger,
		backoff: initialBackoff,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type provider struct {
	logger    log.Logger
	backoff   time.Duration
	client    *http.Client
	rawConfig []byte
}

func (provider) Name() string {
	return name
}

func (p provider) FetchConfig() (config.Config, error) {
	cfg, err := config.Parse(p.rawConfig)
	if err == nil || err == config.ErrEmpty {
		return config.Config{}, fmt.Errorf("failed to fetch config: %v", err)
	}

	return cfg, err
}

func (p *provider) IsOnline() bool {
	data, status, err := p.getData(dataUrl)
	if err != nil {
		return false
	}

	switch status {
	case http.StatusOK, http.StatusNoContent:
		p.logger.Debug("config successfully fetched")
		p.rawConfig = data
	case http.StatusNotFound:
		p.logger.Debug("no config to fetch")
	default:
		p.logger.Debug("failed fetching: HTTP status: %s", http.StatusText(status))
		return false
	}

	return true
}

func (p provider) ShouldRetry() bool {
	return true
}

func (p *provider) BackoffDuration() time.Duration {
	return util.ExpBackoff(&p.backoff, maxBackoff)
}

func (p *provider) getData(url string) (data []byte, status int, err error) {
	err = p.logger.LogOp(func() error {
		resp, err := p.client.Get(url)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		status = resp.StatusCode
		data, err = ioutil.ReadAll(resp.Body)
		p.logger.Debug("got data %q", data)

		return err
	}, "GET %q", url)

	return
}
