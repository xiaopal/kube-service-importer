package prober

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type testStatus bool

func (t testStatus) StatusWeight() int {
	if t {
		return 1
	}
	return -1
}
func (t testStatus) Bool() bool {
	return bool(t)
}

type testProbe func() (bool, error)
type testHealthCheck struct {
	iprobe   int
	probes   []testProbe
	iupdate  int
	updates  []func(bool) error
	status   bool
	statusOK bool
}

func testProbeStatusFunc(probes ...testProbe) ProbeStatusFunc {
	i := 0
	return func(_ context.Context, _ time.Duration) (status interface{}, err error) {
		probe := probes[i]
		if i++; i >= len(probes) {
			i = 0
		}
		bstatus, err := probe()
		return testStatus(bstatus), err
	}
}

func probeSuccess() (bool, error) {
	//fmt.Println("probeSuccess")
	return true, nil
}

func probeFailure() (bool, error) {
	//fmt.Println("probeFailure")
	return false, nil
}

func probeUnknown() (bool, error) {
	//fmt.Println("probeUnknown")
	return false, fmt.Errorf("probe unknown")
}

func TestHeathCheck(t *testing.T) {
	t.Run("n1", func(t *testing.T) {
		check1, check2, check3 := NewStatusProber(t.Name(), testProbeStatusFunc(probeFailure, probeSuccess), nil).
			SetInterval(time.Millisecond).SetTimeout(time.Millisecond).SetRiseCount(2).SetFallCount(2),
			NewStatusProber(t.Name(), testProbeStatusFunc(probeSuccess), nil).
				SetInterval(time.Millisecond).SetTimeout(time.Millisecond).SetRiseCount(1).SetFallCount(1),
			NewStatusProber(t.Name(), testProbeStatusFunc(probeFailure), nil).
				SetInterval(time.Millisecond).SetTimeout(time.Millisecond).SetRiseCount(2).SetFallCount(2)
		StartUpdater(t.Name(), check1)
		//proberOK
		if prober, proberOK := UpdaterGet(t.Name()); !proberOK {
			t.Errorf("probe1: %v, %v", prober, proberOK)
		}
		time.Sleep(100 * time.Millisecond)
		//!statusOK (probe 抖动)
		if status, statusOK := UpdaterStatus(t.Name()); statusOK {
			t.Errorf("status1: %v, %v", status, statusOK)
		}

		StartUpdater(t.Name(), check2)
		time.Sleep(100 * time.Millisecond)
		//status OK
		if status, statusOK := UpdaterStatus(t.Name()); !(statusOK && status.(testStatus).Bool()) {
			t.Errorf("status2: %v, %v", status, statusOK)
		}

		StopUpdater(t.Name())
		//proberOK
		if prober, proberOK := UpdaterGet(t.Name()); proberOK {
			t.Errorf("probe3: %v, %v", prober, proberOK)
		}
		//!statusOK
		if status, statusOK := UpdaterStatus(t.Name()); statusOK {
			t.Errorf("status3: %v, %v", status, statusOK)
		}

		StartUpdater(t.Name(), check3)
		//proberOK
		if prober, proberOK := UpdaterGet(t.Name()); !proberOK {
			t.Errorf("probe4: %v, %v", prober, proberOK)
		}
		time.Sleep(100 * time.Millisecond)
		if status, statusOK := UpdaterStatus(t.Name()); !(statusOK && !status.(testStatus).Bool()) {
			t.Errorf("status4: %v, %v", status, statusOK)
		}
	})
}

func TestHeathCheckFailure(t *testing.T) {
	check3 := NewStatusProber(t.Name(), testProbeStatusFunc(probeFailure), nil).
		SetInterval(time.Millisecond).SetTimeout(time.Millisecond).SetRiseCount(2).SetFallCount(2)
	StartUpdater(t.Name(), check3)
	time.Sleep(100 * time.Millisecond)
	if status, statusOK := UpdaterStatus(t.Name()); !(statusOK && !status.(testStatus).Bool()) {
		t.Errorf("status4: %v, %v", status, statusOK)
	}
}

func TestHeathCheckOnce(t *testing.T) {
	check3 := NewStatusProber(t.Name(), testProbeStatusFunc(probeFailure), NoopOnce).
		SetInterval(time.Millisecond).SetTimeout(time.Millisecond).SetRiseCount(2).SetFallCount(2)
	StartUpdater(t.Name(), check3)
	time.Sleep(100 * time.Millisecond)
	if status, statusOK := UpdaterStatus(t.Name()); statusOK {
		t.Errorf("status4: %v, %v", status, statusOK)
	}
	if status, statusOK := check3.Status(); !(statusOK && !status.(testStatus).Bool()) {
		t.Errorf("status5: %v, %v", status, statusOK)
	}
}
