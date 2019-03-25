package healthcheck

import (
	"context"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// StatusUpdater interface
type StatusUpdater interface {
	Start(key interface{}, check StatusCheck) (loaded bool, stop func())
	Stop(key interface{}) bool
	Status(key interface{}) (status bool, ok bool)
}

// StatusCheck interface
type StatusCheck interface {
	ProbeStatus(context.Context) (bool, error)
	UpdateStatus(bool) error
}

// StatusCheckWithOptions interface
type StatusCheckWithOptions interface {
	StatusCheck
	Interval() time.Duration
	Timeout() time.Duration
	FallCount() int
	RiseCount() int
}

type statusUpdater struct {
	sync.Map
	ctx    context.Context
	logger *log.Logger
}

// NewStatusUpdater func
func NewStatusUpdater(ctx context.Context, logger *log.Logger) StatusUpdater {
	if logger == nil {
		logger = log.New(os.Stderr, "[heathcheck] ", log.Flags())
	}
	return &statusUpdater{
		ctx:    ctx,
		logger: logger,
	}
}

type statusRecord struct {
	atomic.Value
	u         *statusUpdater
	key       interface{}
	ctx       context.Context
	cancelCtx func()
	closed    chan struct{}
	status    int32
}

func (record *statusRecord) Check() StatusCheckWithOptions {
	return record.Load().(StatusCheckWithOptions)
}

func (record *statusRecord) StoreStatus(status bool) {
	if err := record.Check().UpdateStatus(status); err != nil {
		record.u.logger.Printf("update status: %v", err)
		return
	}
	if status {
		atomic.StoreInt32(&record.status, 1)
		return
	}
	atomic.StoreInt32(&record.status, -1)
}

func (record *statusRecord) LoadStatus() (status bool, ok bool) {
	istatus := atomic.LoadInt32(&record.status)
	return istatus > 0, istatus != 0
}

func (u *statusUpdater) stopRecord(record *statusRecord) {
	record.cancelCtx()
	<-record.closed
}

func (u *statusUpdater) Start(key interface{}, check StatusCheck) (loaded bool, stop func()) {
	if check == nil {
		if val, loaded := u.Load(key); loaded {
			u.stopRecord(val.(*statusRecord))
			return true, nil
		}
		return false, nil
	}
	val, loaded := u.LoadOrStore(key, &statusRecord{u: u, key: key})
	record := val.(*statusRecord)
	record.Store(WithOptions(check, 10*time.Second, 10*time.Second, 1, 1, false))
	stop = func() {
		u.stopRecord(record)
	}
	if !loaded {
		record.ctx, record.cancelCtx = context.WithCancel(u.ctx)
		record.closed = make(chan struct{})
		go func() {
			defer func() {
				if err := recover(); err != nil {
					u.logger.Printf("panic: %v", err)
				}
				u.Delete(record.key)
				close(record.closed)
				stop()
			}()
			success, failure, timer := 0, 0, time.NewTimer(record.Check().Interval())
			defer timer.Stop()
			probe := func() (status bool, statusOK bool) {
				check := record.Check()
				timer.Reset(check.Interval())
				ctx, cancel := context.WithTimeout(u.ctx, check.Timeout())
				defer cancel()
				status, err := check.ProbeStatus(ctx)
				switch {
				case err != nil:
					u.logger.Printf("probe: %v", err)
					return false, false
				case status:
					success, failure = success+1, 0
					statusOK = success >= check.RiseCount()
				case !status:
					success, failure = 0, failure+1
					statusOK = failure >= check.FallCount()
				}
				return status, statusOK
			}
			for {
				select {
				case <-record.ctx.Done():
					return
				case <-timer.C:
					status, statusOK := record.LoadStatus()
					if probeStatus, probeOK := probe(); probeOK && (!statusOK || status != probeStatus) {
						record.StoreStatus(probeStatus)
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

func (u *statusUpdater) Status(key interface{}) (status bool, ok bool) {
	if val, loaded := u.Load(key); loaded {
		return val.(*statusRecord).LoadStatus()
	}
	return false, false
}

type statusCheckOptionsWrapper struct {
	StatusCheck
	overwrite bool
	interval  time.Duration
	timeout   time.Duration
	fallCount int
	riseCount int
}

// WithOptions func
func WithOptions(check StatusCheck, interval time.Duration, timeout time.Duration, fallCount int, riseCount int, overwrite bool) StatusCheckWithOptions {
	return &statusCheckOptionsWrapper{
		StatusCheck: check,
		overwrite:   overwrite,
		interval:    interval,
		timeout:     timeout,
		fallCount:   fallCount,
		riseCount:   riseCount,
	}
}

func (w *statusCheckOptionsWrapper) Interval() time.Duration {
	if o, ok := w.StatusCheck.(StatusCheckWithOptions); ok && !w.overwrite {
		if val := o.Interval(); val > 0 {
			return val
		}
	}
	return w.interval
}
func (w *statusCheckOptionsWrapper) Timeout() time.Duration {
	if o, ok := w.StatusCheck.(StatusCheckWithOptions); ok && !w.overwrite {
		if val := o.Timeout(); val > 0 {
			return val
		}
	}
	return w.timeout
}
func (w *statusCheckOptionsWrapper) FallCount() int {
	if o, ok := w.StatusCheck.(StatusCheckWithOptions); ok && !w.overwrite {
		if val := o.FallCount(); val > 0 {
			return val
		}
	}
	return w.fallCount
}
func (w *statusCheckOptionsWrapper) RiseCount() int {
	if o, ok := w.StatusCheck.(StatusCheckWithOptions); ok && !w.overwrite {
		if val := o.RiseCount(); val > 0 {
			return val
		}
	}
	return w.riseCount
}

// DefaultStatusUpdater var
var DefaultStatusUpdater = NewStatusUpdater(context.TODO(), nil)

// Start func
func Start(key interface{}, check StatusCheck) (loaded bool, stop func()) {
	return DefaultStatusUpdater.Start(key, check)
}

// Stop func
func Stop(key interface{}) bool {
	return DefaultStatusUpdater.Stop(key)
}

// Status func
func Status(key interface{}) (status bool, ok bool) {
	return DefaultStatusUpdater.Status(key)
}
