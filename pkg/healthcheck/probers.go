package healthcheck

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/xiaopal/kube-service-importer/pkg/fluconf"
	"k8s.io/kubernetes/pkg/probe"
	httpprobe "k8s.io/kubernetes/pkg/probe/http"
	tcpprobe "k8s.io/kubernetes/pkg/probe/tcp"
)

// StatusCheckProber type
type StatusCheckProber func(context.Context) (bool, error)

// StatusCheckUpdater type
type StatusCheckUpdater func(bool) error

// SimpleStatusCheck type
type SimpleStatusCheck struct {
	Probers  []StatusCheckProber
	Updaters []StatusCheckUpdater
}

// ProbeStatus func
func (c *SimpleStatusCheck) ProbeStatus(ctx context.Context) (bool, error) {
	for _, prober := range c.Probers {
		status, err := prober(ctx)
		switch {
		case err != nil:
			return false, err
		case !status:
			return false, nil
		}
	}
	return true, nil
}

// UpdateStatus func
func (c *SimpleStatusCheck) UpdateStatus(status bool) error {
	for _, updater := range c.Updaters {
		if err := updater(status); err != nil {
			return err
		}
	}
	return nil
}

func newHTTPProber(conf fluconf.Config) (StatusCheckProber, error) {
	p, host, port, uri, timeout := httpprobe.New(), conf.GetString("host", "127.0.0.1"), conf.GetInt("port", 80), conf.GetString("uri", "/"), conf.GetDuration("timeout", 10*time.Second)
	if port <= 0 {
		return nil, fmt.Errorf("illegal port: %v", port)
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("illegal timeout: %v", timeout)
	}
	u, err := url.Parse(fmt.Sprintf("http://%s:%d/", host, port))
	if err != nil {
		return nil, fmt.Errorf("illegal host or port: %s, %d", host, port)
	}
	u, err = u.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("illegal uri: %s", uri)
	}

	return func(_ context.Context) (bool, error) {
		status, _, err := p.Probe(u, http.Header{}, timeout)
		if err != nil || status == probe.Unknown {
			return false, fmt.Errorf("http probe: %v, %v", status, err)
		}
		return (status == probe.Success), nil
	}, nil
}

func newTCPProber(conf fluconf.Config) (StatusCheckProber, error) {
	p, host, port, timeout := tcpprobe.New(), conf.GetString("host", "127.0.0.1"), conf.GetInt("port", 0), conf.GetDuration("timeout", 10*time.Second)
	if port <= 0 {
		return nil, fmt.Errorf("illegal port: %v", port)
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("illegal timeout: %v", timeout)
	}
	return func(_ context.Context) (bool, error) {
		status, _, err := p.Probe(host, port, timeout)
		if err != nil || status == probe.Unknown {
			return false, fmt.Errorf("tcp probe: %v, %v", status, err)
		}
		return (status == probe.Success), nil
	}, nil
}

var proberFactories = map[string]func(conf fluconf.Config) (StatusCheckProber, error){
	"http": newHTTPProber,
	"tcp":  newTCPProber,
}

// NewProber from config
func NewProber(conf fluconf.Config) (StatusCheckProber, error) {
	if factory, ok := proberFactories[conf["probe"]]; ok {
		return factory(conf)
	}
	return nil, fmt.Errorf("illegal probe config: %v", conf)
}

// NewProberSafe from config
func NewProberSafe(conf fluconf.Config) StatusCheckProber {
	prober, err := NewProber(conf)
	if err != nil {
		return func(_ context.Context) (bool, error) {
			return false, err
		}
	}
	return prober
}

// NewStatusCheckWithUpdaters config
func NewStatusCheckWithUpdaters(conf fluconf.Config, updaters ...StatusCheckUpdater) StatusCheck {
	interval, timeout, riseCount, fallCount := conf.GetDuration("interval", 0), conf.GetDuration("timeout", 0), conf.GetInt("rise", 0), conf.GetInt("fall", 0)
	check := &SimpleStatusCheck{Probers: []StatusCheckProber{NewProberSafe(conf)}, Updaters: updaters}
	return WithOptions(check, interval, timeout, fallCount, riseCount, false)
}
