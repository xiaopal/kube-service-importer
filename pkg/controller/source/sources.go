package source

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/xiaopal/kube-service-importer/pkg/fluconf"
	"github.com/xiaopal/kube-service-importer/pkg/prober"
)

type (
	// LoadFunc type
	LoadFunc func(context.Context, time.Duration, *log.Logger) (*LoadResult, error)
	// LoadResult type
	LoadResult struct {
		IPs       []string
		Ports     []int
		Protocol  string
		Overwrite bool
	}
)

// Loader func
func Loader(conf fluconf.Config, updateFunc func(*LoadResult), logger *log.Logger) (prober.StatusProber, error) {
	factory, ok := SourceFuncFactories[conf["source"]]
	if !ok {
		return nil, fmt.Errorf("illegal import config: %v", conf)
	}
	loader, name, err := factory(conf)
	if err != nil {
		return nil, err
	}
	loadSource, updateSource := func(ctx context.Context, timeout time.Duration) (interface{}, error) {
		result, err := loader(ctx, timeout, logger)
		switch {
		case err != nil:
			return nil, err
		case result == nil:
			return nil, prober.ErrorStatusUnknown
		default:
			return result, nil
		}
	}, func(status interface{}) error {
		if updateFunc != nil {
			updateFunc(status.(*LoadResult))
		}
		return nil
	}
	source := prober.NewStatusProber(conf.GetString("name", name), loadSource, updateSource)
	source.SetInterval(conf.GetDuration("interval", 30*time.Second))
	source.SetTimeout(conf.GetDuration("timeout", 30*time.Second))
	return source, nil
}

// SourceFuncFactories var
var SourceFuncFactories = map[string]func(conf fluconf.Config) (source LoadFunc, name string, err error){
	"static":   staticSourceLoader,
	"nslookup": nslookupSourceLoader,
}
