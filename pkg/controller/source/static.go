package source

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/xiaopal/kube-service-importer/pkg/fluconf"
)

func staticSourceLoader(conf fluconf.Config) (LoadFunc, string, error) {
	ips, port, protocol, overwrite := strings.Split(conf.GetString("ip", ""), ","),
		conf.GetInt("port", 0),
		strings.ToUpper(conf.GetString("protocol", "TCP")),
		conf.GetBool("overwrite", false)
	if port <= 0 {
		return nil, "", fmt.Errorf("illegal port %v", port)
	}
	return func(context.Context, time.Duration, *log.Logger) (*LoadResult, error) {
		return &LoadResult{
			IPs:       ips,
			Ports:     []int{port},
			Protocol:  protocol,
			Overwrite: overwrite,
		}, nil
	}, fmt.Sprintf("static|%s:%d/%s", strings.Join(ips, ","), port, protocol), nil
}
