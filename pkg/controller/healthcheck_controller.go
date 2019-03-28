package controller

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"github.com/xiaopal/kube-informer/pkg/informer"
	"github.com/xiaopal/kube-informer/pkg/kubeclient"
	"github.com/xiaopal/kube-service-importer/pkg/prober"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
)

// EndpointHealthCheckController interface
type EndpointHealthCheckController interface{}

type endpointHealthCheckController struct {
	ctx                     context.Context
	labelSelector           string
	annotationProbes        string
	annotationProbesStarted string
	logger                  *log.Logger
	informer                informer.Informer
	updateQueue             workqueue.RateLimitingInterface
}

// StartHealthCheckController func
func StartHealthCheckController(ctx context.Context, kubeClient kubeclient.Client, labelSelector string, annotationProbes string) (EndpointHealthCheckController, error) {
	config, err := kubeClient.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get config: %v", err)
	}
	if labelSelector == "" {
		return nil, fmt.Errorf("labelSelector required")
	}
	if annotationProbes == "" {
		return nil, fmt.Errorf("annotationProbes required")
	}
	logger := log.New(os.Stderr, "[heathcheck] ", log.Flags())
	c := &endpointHealthCheckController{
		ctx:                     ctx,
		labelSelector:           labelSelector,
		annotationProbes:        annotationProbes,
		annotationProbesStarted: fmt.Sprintf("%s-started", annotationProbes),
		logger:                  logger,
		updateQueue:             workqueue.NewRateLimitingQueue(informer.DefaultRateLimiter(5*time.Millisecond, 1000*time.Second, math.MaxFloat64, math.MaxInt32)),
	}

	c.informer = informer.NewInformer(config, informer.Opts{
		Logger:     logger,
		MaxRetries: -1,
		Handler: func(ctx context.Context, event informer.EventType, obj *unstructured.Unstructured, numRetries int) error {
			return c.handleEvent(ctx, event, obj)
		},
	})
	c.informer.Watch("v1", "Endpoint", kubeClient.Namespace(), labelSelector, "", 600*time.Second)
	go wait.Until(func() {
		for c.processNext(ctx) {
		}
	}, time.Second, ctx.Done())
	return c, c.informer.Run(ctx)
}

func (c *endpointHealthCheckController) handleEvent(ctx context.Context, event informer.EventType, obj *unstructured.Unstructured) error {
	switch event {
	case informer.EventAdd, informer.EventUpdate:
		prober.Start(nil, nil)
	case informer.EventDelete:
		prober.Stop(nil)
	}
	return nil
}

func (c *endpointHealthCheckController) updateStatus(key interface{}, status bool) error {
	return nil
}

func (c *endpointHealthCheckController) processNext(ctx context.Context) bool {
	item, quit := c.updateQueue.Get()
	if quit {
		return false
	}
	defer c.updateQueue.Done(item)
	c.updateQueue.Forget(item)
	return true
}
