package healthcheck

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type testProbe func() (bool, error)
type testHealthCheck struct {
	iprobe   int
	probes   []testProbe
	iupdate  int
	updates  []func(bool) error
	status   bool
	statusOK bool
}

func (c *testHealthCheck) ProbeStatus(_ context.Context) (status bool, err error) {
	probe := c.probes[c.iprobe]
	if c.iprobe++; c.iprobe >= len(c.probes) {
		c.iprobe = 0
	}
	return probe()
}

func (c *testHealthCheck) UpdateStatus(status bool) error {
	c.status, c.statusOK = status, true
	if len(c.updates) > 0 {
		update := c.updates[c.iupdate]
		if c.iupdate++; c.iupdate >= len(c.updates) {
			c.iupdate = 0
		}
		return update(status)
	}
	return nil
}

func probeSuccess() (bool, error) {
	return true, nil
}

func probeFailure() (bool, error) {
	return false, nil
}

func probeUnknown() (bool, error) {
	return false, fmt.Errorf("probe unknown")
}

func TestHeathCheck(t *testing.T) {
	t.Run("n1", func(t *testing.T) {
		check1, check2, check3 := &testHealthCheck{
			probes: []testProbe{probeFailure, probeSuccess},
		}, &testHealthCheck{
			probes: []testProbe{probeSuccess},
		}, &testHealthCheck{
			probes: []testProbe{probeFailure},
		}
		Start(t.Name(), WithOptions(check1, time.Millisecond, time.Millisecond, 2, 2, false))
		time.Sleep(100 * time.Millisecond)
		//!statusOK (probe 抖动)
		if status, statusOK := Status(t.Name()); statusOK {
			t.Errorf("status1: %v, %v", status, statusOK)
		}

		Start(t.Name(), WithOptions(check2, time.Millisecond, time.Millisecond, 1, 1, false))
		time.Sleep(100 * time.Millisecond)
		//status OK
		if status, statusOK := Status(t.Name()); !(status && statusOK) {
			t.Errorf("status2: %v, %v", status, statusOK)
		}

		Stop(t.Name())
		//!statusOK
		if status, statusOK := Status(t.Name()); statusOK {
			t.Errorf("status3: %v, %v", status, statusOK)
		}

		Start(t.Name(), WithOptions(check3, time.Millisecond, time.Millisecond, 2, 2, false))
		time.Sleep(100 * time.Millisecond)
		if status, statusOK := Status(t.Name()); !(!status && statusOK) {
			t.Errorf("status4: %v, %v", status, statusOK)
		}
	})
}
