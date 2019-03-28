package prober

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/xiaopal/kube-service-importer/pkg/fluconf"
	kprobe "k8s.io/kubernetes/pkg/probe"
	httpprobe "k8s.io/kubernetes/pkg/probe/http"
	tcpprobe "k8s.io/kubernetes/pkg/probe/tcp"
)

// SimpleStatusProbeFunc type
type SimpleStatusProbeFunc func(context.Context, time.Duration) (bool, error)

// SimpleStatusUpdateFunc type
type SimpleStatusUpdateFunc func(bool) error

// SimpleStatusProber type
type simpleStatusProber struct {
	StatusProber
	Probers  []SimpleStatusProbeFunc
	Updaters []SimpleStatusUpdateFunc
}

// SimpleStatusResult type
type SimpleStatusResult bool

// StatusWeight func
func (r SimpleStatusResult) StatusWeight() int {
	if r {
		return 1
	}
	return -1
}

// NewSimpleStatusProber func
func NewSimpleStatusProber(probers []SimpleStatusProbeFunc, updaters []SimpleStatusUpdateFunc) StatusProber {
	return NewStatusProber(func(ctx context.Context, timeout time.Duration) (interface{}, error) {
		for _, prober := range probers {
			status, err := prober(ctx, timeout)
			switch {
			case err != nil:
				return nil, err
			case !bool(status):
				return SimpleStatusResult(false), nil
			}
		}
		return SimpleStatusResult(true), nil
	}, func(status interface{}) error {
		bstatus := bool(status.(SimpleStatusResult))
		for _, updater := range updaters {
			if err := updater(bstatus); err != nil {
				return err
			}
		}
		return nil
	})
}

func httpProbeFunc(conf fluconf.Config) (SimpleStatusProbeFunc, error) {
	p, host, port, uri := httpprobe.New(), conf.GetString("host", "127.0.0.1"), conf.GetInt("port", 80), conf.GetString("uri", "/")
	if port <= 0 {
		return nil, fmt.Errorf("illegal port: %v", port)
	}
	u, err := url.Parse(fmt.Sprintf("http://%s:%d/", host, port))
	if err != nil {
		return nil, fmt.Errorf("illegal host or port: %s, %d", host, port)
	}
	u, err = u.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("illegal uri: %s", uri)
	}

	return func(_ context.Context, timeout time.Duration) (bool, error) {
		status, _, err := p.Probe(u, http.Header{}, timeout)
		if err != nil || status == kprobe.Unknown {
			return false, fmt.Errorf("http probe: %v, %v", status, err)
		}
		return (status == kprobe.Success), nil
	}, nil
}

func tcpProbeFunc(conf fluconf.Config) (SimpleStatusProbeFunc, error) {
	p, host, port := tcpprobe.New(), conf.GetString("host", "127.0.0.1"), conf.GetInt("port", 0)
	if port <= 0 {
		return nil, fmt.Errorf("illegal port: %v", port)
	}
	return func(_ context.Context, timeout time.Duration) (bool, error) {
		status, _, err := p.Probe(host, port, timeout)
		if err != nil || status == kprobe.Unknown {
			return false, fmt.Errorf("tcp probe: %v, %v", status, err)
		}
		return (status == kprobe.Success), nil
	}, nil
}

var proberFactories = map[string]func(conf fluconf.Config) (SimpleStatusProbeFunc, error){
	"http": httpProbeFunc,
	"tcp":  tcpProbeFunc,
}

// LoadSimpleStatusProbeFunc from config
func LoadSimpleStatusProbeFunc(conf fluconf.Config) (SimpleStatusProbeFunc, error) {
	if factory, ok := proberFactories[conf["probe"]]; ok {
		return factory(conf)
	}
	return nil, fmt.Errorf("illegal probe config: %v", conf)
}

// LoadSimpleStatusProbeFuncSafe from config
func LoadSimpleStatusProbeFuncSafe(conf fluconf.Config) SimpleStatusProbeFunc {
	prober, err := LoadSimpleStatusProbeFunc(conf)
	if err != nil {
		return func(_ context.Context, timeout time.Duration) (bool, error) {
			return false, err
		}
	}
	return prober
}

// LoadSimpleStatusProber config
func LoadSimpleStatusProber(conf fluconf.Config, updaters ...SimpleStatusUpdateFunc) StatusProber {
	prober := NewSimpleStatusProber([]SimpleStatusProbeFunc{LoadSimpleStatusProbeFuncSafe(conf)}, updaters)
	prober.SetInterval(conf.GetDuration("interval", prober.Interval()))
	prober.SetTimeout(conf.GetDuration("timeout", prober.Timeout()))
	prober.SetRiseCount(conf.GetInt("rise", prober.RiseCount()))
	prober.SetFallCount(conf.GetInt("fall", prober.FallCount()))
	return prober
}
