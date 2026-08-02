// Copyright (C) 2024 Plane Watch
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This file is part of pw-feeder.
//
// pw-feeder is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// pw-feeder is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with pw-feeder. If not, see <https://www.gnu.org/licenses/>.

package backoff

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestBackerOff_BackOff verifies delay calculation and inactivity resets.
func TestBackerOff_BackOff(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		bo := New(WithMethod(DefaultMethodExponentialBackoff), WithResetAfter(30*time.Second))
		for i := 0; i <= 5; i++ {
			assert.Equal(t, time.Duration(0*time.Second), bo.BackOff())
			assert.Equal(t, time.Duration(1*time.Second), bo.BackOff())
			assert.Equal(t, time.Duration(4*time.Second), bo.BackOff())
			assert.Equal(t, time.Duration(9*time.Second), bo.BackOff())
			assert.Equal(t, time.Duration(16*time.Second), bo.BackOff())
			assert.Equal(t, time.Duration(25*time.Second), bo.BackOff())
			assert.Equal(t, time.Duration(30*time.Second), bo.BackOff())
			assert.Equal(t, time.Duration(30*time.Second), bo.BackOff())
			time.Sleep(31 * time.Second)
		}
	})
}
