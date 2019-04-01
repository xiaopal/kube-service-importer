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
type SimpleStatusProbeFunc func(context.Context, time.Duration) (int, error)

// SimpleStatusUpdateFunc type
type SimpleStatusUpdateFunc func(int) error

// SimpleStatusResult type
type SimpleStatusResult int

// StatusWeight func
func (r SimpleStatusResult) StatusWeight() int {
	return int(r)
}

// NewSimpleStatusProber func
func NewSimpleStatusProber(name string, probes []SimpleStatusProbeFunc, updates []SimpleStatusUpdateFunc) StatusProber {
	return NewStatusProber(name, func(ctx context.Context, timeout time.Duration) (interface{}, error) {
		status := 0
		for _, probe := range probes {
			probeStatus, err := probe(ctx, timeout)
			if err != nil {
				return nil, err
			}
			status += probeStatus
		}
		return SimpleStatusResult(status), nil
	}, func(status interface{}) error {
		istatus := int(status.(SimpleStatusResult))
		for _, update := range updates {
			if err := update(istatus); err != nil {
				return err
			}
		}
		return nil
	})
}

// DefaultSimpleStatusResult convert bool to SimpleStatusResult
func DefaultSimpleStatusResult(status bool, ok bool) SimpleStatusResult {
	if !ok {
		return SimpleStatusResult(0)
	}
	if status {
		return SimpleStatusResult(1)
	}
	return SimpleStatusResult(-1)
}

func kprobeResultWeight(result kprobe.Result, conf fluconf.Config) int {
	return conf.GetInt(string(result), DefaultSimpleStatusResult(result == kprobe.Success, true).StatusWeight())
}

func loadHTTPProbeFunc(conf fluconf.Config) (SimpleStatusProbeFunc, string, error) {
	p, host, port, uri := httpprobe.New(false), conf.GetString("host", "127.0.0.1"), conf.GetInt("port", 80), conf.GetString("uri", "/")
	if port <= 0 {
		return nil, "", fmt.Errorf("illegal port: %v", port)
	}
	u, err := url.Parse(fmt.Sprintf("http://%s:%d/", host, port))
	if err != nil {
		return nil, "", fmt.Errorf("illegal host or port: %s, %d", host, port)
	}
	u, err = u.Parse(uri)
	if err != nil {
		return nil, "", fmt.Errorf("illegal uri: %s", uri)
	}

	return func(_ context.Context, timeout time.Duration) (int, error) {
		status, _, err := p.Probe(u, http.Header{}, timeout)
		if err != nil || status == kprobe.Unknown {
			return kprobeResultWeight(kprobe.Unknown, conf), nil
		}
		return kprobeResultWeight(status, conf), nil
	}, fmt.Sprintf("http|%s", u), nil
}

func loadTCPProbeFunc(conf fluconf.Config) (SimpleStatusProbeFunc, string, error) {
	p, host, port := tcpprobe.New(), conf.GetString("host", "127.0.0.1"), conf.GetInt("port", 0)
	if port <= 0 {
		return nil, "", fmt.Errorf("illegal port: %v", port)
	}
	return func(_ context.Context, timeout time.Duration) (int, error) {
		status, _, err := p.Probe(host, port, timeout)
		if err != nil || status == kprobe.Unknown {
			return kprobeResultWeight(kprobe.Unknown, conf), nil
		}
		return kprobeResultWeight(status, conf), nil
	}, fmt.Sprintf("tcp|%s:%d", host, port), nil
}

// SimpleStatusProbeFuncFactories var
var SimpleStatusProbeFuncFactories = map[string]func(conf fluconf.Config) (SimpleStatusProbeFunc, string, error){
	"http": loadHTTPProbeFunc,
	"tcp":  loadTCPProbeFunc,
}

// LoadSimpleStatusProbeFunc from config
func LoadSimpleStatusProbeFunc(conf fluconf.Config) (SimpleStatusProbeFunc, string, error) {
	factory, ok := SimpleStatusProbeFuncFactories[conf["probe"]]
	if !ok {
		return nil, "", fmt.Errorf("illegal probe config: %v", conf)
	}
	f, name, err := factory(conf)
	if err != nil {
		return nil, "", err
	}
	return f, conf.GetString("name", name), err
}

// LoadSimpleStatusProbeFuncSafe from config
func LoadSimpleStatusProbeFuncSafe(conf fluconf.Config) (SimpleStatusProbeFunc, string) {
	prober, name, err := LoadSimpleStatusProbeFunc(conf)
	if err != nil {
		return func(_ context.Context, timeout time.Duration) (int, error) {
			return 0, err
		}, ""
	}
	return prober, name
}

// LoadSimpleStatusProber config
func LoadSimpleStatusProber(conf fluconf.Config, updaters ...SimpleStatusUpdateFunc) StatusProber {
	probe, name := LoadSimpleStatusProbeFuncSafe(conf)
	prober := NewSimpleStatusProber(name, []SimpleStatusProbeFunc{probe}, updaters)
	prober.SetInterval(conf.GetDuration("interval", prober.Interval()))
	prober.SetTimeout(conf.GetDuration("timeout", prober.Timeout()))
	prober.SetRiseCount(conf.GetInt("rise", prober.RiseCount()))
	prober.SetFallCount(conf.GetInt("fall", prober.FallCount()))
	return prober
}

// LoadSimpleStatusProbers func
func LoadSimpleStatusProbers(confs []fluconf.Config, updaters ...SimpleStatusUpdateFunc) []StatusProber {
	probers := make([]StatusProber, len(confs))
	for i, conf := range confs {
		probers[i] = LoadSimpleStatusProber(conf)
	}
	return probers
}
