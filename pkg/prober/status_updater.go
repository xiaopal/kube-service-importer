package prober

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/glog"
)

// StatusUpdater interface
type StatusUpdater interface {
	Start(key interface{}, prober StatusProber) (loaded bool, stop func())
	Stop(key interface{}) bool
	Status(key interface{}) (status interface{}, ok bool)
	Get(key interface{}) (prober StatusProber, ok bool)
}

// StatusWeight interface
type StatusWeight interface {
	StatusWeight() int
}

// StatusProber interface
type StatusProber interface {
	Name() string
	ProbeStatus(ctx context.Context, timeout time.Duration) (status interface{}, err error)
	UpdateStatus(status interface{}) error
	Status() (status interface{}, ok bool)

	Interval() time.Duration
	Timeout() time.Duration
	FallCount() int
	RiseCount() int

	SetInterval(val time.Duration) StatusProber
	SetTimeout(val time.Duration) StatusProber
	SetFallCount(val int) StatusProber
	SetRiseCount(val int) StatusProber
}

// ProbeStatusFunc type
type ProbeStatusFunc func(context.Context, time.Duration) (interface{}, error)

// UpdateStatusFunc type
type UpdateStatusFunc func(interface{}) error

var (
	// ErrorAbort abort silently
	ErrorAbort = errors.New("Abort silently")
	// ErrorStatusUnknown unknown status silently
	ErrorStatusUnknown = errors.New("Unknown status silently")
)

// UpdateOnce func
func UpdateOnce(updateStatus UpdateStatusFunc) UpdateStatusFunc {
	return func(status interface{}) error {
		if updateStatus != nil {
			if err := updateStatus(status); err != nil {
				return err
			}
		}
		return ErrorAbort
	}
}

// NoopOnce func
var NoopOnce = UpdateOnce(nil)

// StatusProber interface
type statusProber struct {
	atomic.Value
	name         string
	stored       int32
	probeStatus  ProbeStatusFunc
	updateStatus UpdateStatusFunc
	interval     time.Duration
	timeout      time.Duration
	riseCount    int
	fallCount    int
}

func (p *statusProber) Name() string {
	return p.name
}

func (p *statusProber) ProbeStatus(ctx context.Context, timeout time.Duration) (interface{}, error) {
	return p.probeStatus(ctx, timeout)
}

func (p *statusProber) UpdateStatus(status interface{}) error {
	p.Store(status)
	atomic.StoreInt32(&p.stored, 1)
	if p.updateStatus != nil {
		if err := p.updateStatus(status); err != nil {
			return err
		}
	}
	return nil
}

func (p *statusProber) Status() (interface{}, bool) {
	return p.Load(), atomic.LoadInt32(&p.stored) > 0
}

func (p *statusProber) Interval() time.Duration {
	return p.interval
}

func (p *statusProber) Timeout() time.Duration {
	return p.timeout
}

func (p *statusProber) FallCount() int {
	return p.fallCount
}

func (p *statusProber) RiseCount() int {
	return p.riseCount
}

func (p *statusProber) SetInterval(val time.Duration) StatusProber {
	p.interval = val
	return p
}

func (p *statusProber) SetTimeout(val time.Duration) StatusProber {
	p.timeout = val
	return p
}

func (p *statusProber) SetFallCount(val int) StatusProber {
	p.fallCount = val
	return p
}

func (p *statusProber) SetRiseCount(val int) StatusProber {
	p.riseCount = val
	return p
}

func (p *statusProber) String() string {
	return fmt.Sprintf("probe: %v interval=%v timeout=%v rise=%v fall=%v", p.Name(), p.Interval(), p.Timeout(), p.RiseCount(), p.FallCount())
}

// NewStatusProber func
func NewStatusProber(name string, probeStatus ProbeStatusFunc, updateStatus UpdateStatusFunc) StatusProber {
	return &statusProber{
		name:         name,
		probeStatus:  probeStatus,
		updateStatus: updateStatus,
		interval:     10 * time.Second,
		timeout:      10 * time.Second,
		riseCount:    1,
		fallCount:    1,
	}
}

type statusUpdater struct {
	sync.Map
	ctx    context.Context
	logger *log.Logger
}

// NewStatusUpdater func
func NewStatusUpdater(ctx context.Context, logger *log.Logger) StatusUpdater {
	if logger == nil {
		logger = log.New(os.Stderr, "[status-updater] ", log.Flags())
	}
	return &statusUpdater{
		ctx:    ctx,
		logger: logger,
	}
}

type statusRecord struct {
	atomic.Value
	u             *statusUpdater
	key           interface{}
	ctx           context.Context
	cancelCtx     func()
	closed        chan struct{}
	status        atomic.Value
	statuesStored int32
}

func (record *statusRecord) Prober() StatusProber {
	return record.Load().(StatusProber)
}

func (record *statusRecord) StoreStatus(status interface{}) (abort bool) {
	if err := record.Prober().UpdateStatus(status); err != nil {
		if err == ErrorAbort {
			return true
		}
		record.u.logger.Printf("update status (%v): %v", record.Prober(), err)
		return false
	}
	record.status.Store(status)
	atomic.StoreInt32(&record.statuesStored, 1)
	return false
}

func (record *statusRecord) LoadStatus() (status interface{}, ok bool) {
	return record.status.Load(), atomic.LoadInt32(&record.statuesStored) > 0
}

func (u *statusUpdater) stopRecord(record *statusRecord) {
	record.cancelCtx()
	<-record.closed
}

func (u *statusUpdater) Start(key interface{}, prober StatusProber) (loaded bool, stop func()) {
	if prober == nil {
		if val, loaded := u.Load(key); loaded {
			u.stopRecord(val.(*statusRecord))
			return true, nil
		}
		return false, nil
	}
	val, loaded := u.LoadOrStore(key, &statusRecord{u: u, key: key})
	record := val.(*statusRecord)
	record.Store(prober)
	stop = func() {
		u.stopRecord(record)
	}
	if !loaded {
		record.ctx, record.cancelCtx = context.WithCancel(u.ctx)
		record.closed = make(chan struct{})
		go func() {
			defer func() {
				if err := recover(); err != nil {
					u.logger.Printf("PANAC!!! (%v|%v): %v\n%v", record.key, record.Prober(), err, string(debug.Stack()))
				}
				u.Delete(record.key)
				close(record.closed)
				stop()
			}()
			success, failure, timer := 0, 0, time.NewTimer(time.Millisecond)
			defer timer.Stop()
			doProbe := func() (status interface{}, statusOK bool, abort bool) {
				prober := record.Prober()
				if interval := prober.Interval(); interval > 0 {
					timer.Reset(interval)
				} else {
					abort = true
				}
				ctx, cancel := u.ctx, context.CancelFunc(nil)
				if timeout := prober.Timeout(); timeout > 0 {
					ctx, cancel = context.WithTimeout(u.ctx, prober.Timeout())
					defer cancel()
				}
				status, err := prober.ProbeStatus(ctx, prober.Timeout())
				if glog.V(2) || (err != nil && err != ErrorStatusUnknown) {
					record.u.logger.Printf("probe (%v): status=%v, err=%v", prober, status, err)
				}
				if err != nil {
					return nil, false, abort || err == ErrorAbort
				}
				if weight, weightOK := status.(StatusWeight); weightOK {
					w := weight.StatusWeight()
					switch {
					case w > 0:
						success, failure = success+w, 0
						statusOK = success >= prober.RiseCount()
					case w < 0:
						success, failure = 0, failure+w
						statusOK = failure <= -prober.FallCount()
					}
				} else {
					success, failure, statusOK = 0, 0, true
				}
				return status, statusOK, abort
			}
			for {
				select {
				case <-record.ctx.Done():
					return
				case <-timer.C:
					status, statusOK := record.LoadStatus()
					probeStatus, probeOK, abort := doProbe()
					if probeOK && (!statusOK || status != probeStatus) {
						abort = record.StoreStatus(probeStatus) || abort
					}
					if abort {
						return
					}
				}
			}
		}()
	}
	return loaded, stop
}

func (u *statusUpdater) Stop(key interface{}) bool {
	loaded, _ := u.Start(key, nil)
	return loaded
}

func (u *statusUpdater) Status(key interface{}) (status interface{}, ok bool) {
	if val, loaded := u.Load(key); loaded {
		return val.(*statusRecord).LoadStatus()
	}
	return false, false
}

func (u *statusUpdater) Get(key interface{}) (prober StatusProber, ok bool) {
	if val, loaded := u.Load(key); loaded {
		return val.(*statusRecord).Prober(), true
	}
	return nil, false
}

// DefaultStatusUpdater var
var DefaultStatusUpdater = NewStatusUpdater(context.TODO(), nil)

// StartUpdater func
func StartUpdater(key interface{}, prober StatusProber) (loaded bool, stop func()) {
	return DefaultStatusUpdater.Start(key, prober)
}

// StopUpdater func
func StopUpdater(key interface{}) bool {
	return DefaultStatusUpdater.Stop(key)
}

// UpdaterStatus func
func UpdaterStatus(key interface{}) (status interface{}, ok bool) {
	return DefaultStatusUpdater.Status(key)
}

// UpdaterGet func
func UpdaterGet(key interface{}) (prober StatusProber, ok bool) {
	return DefaultStatusUpdater.Get(key)
}
