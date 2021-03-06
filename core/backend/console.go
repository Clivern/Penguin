// Copyright 2020 Clivern. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package backend

import (
	"fmt"

	"github.com/clivern/penguin/core/model"

	log "github.com/sirupsen/logrus"
)

// Console struct
type Console struct{}

// NewConsole create a new instance of console backend
func NewConsole() *Console {
	return &Console{}
}

// Send sends metrics to console backend
func (c *Console) Send(metrics []model.Metric) error {
	log.Info(fmt.Sprintf(
		"Send %d metrics to console backend",
		len(metrics),
	))

	return nil
}
