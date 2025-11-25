package backoff

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
)

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
