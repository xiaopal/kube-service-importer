package controller

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"github.com/xiaopal/kube-service-importer/pkg/fluconf"
	"github.com/xiaopal/kube-service-importer/pkg/prober"

	"github.com/xiaopal/kube-informer/pkg/informer"
	"github.com/xiaopal/kube-informer/pkg/kubeclient"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	dynamic "k8s.io/client-go/deprecated-dynamic"
)

// EndpointsImporter interface
type EndpointsImporter interface{}

type endpointsImporter struct {
	ctx                                 context.Context
	statusUpdater                       prober.StatusUpdater
	kubeClient                          kubeclient.Client
	client                              dynamic.Interface
	resource                            *metav1.APIResource
	labelSelector                       string
	annotationSources, annotationProbes string
	logger                              *log.Logger
	informer                            informer.Informer
	targets                             map[objectKey]*targetRecord
	updateQueue                         workqueue.RateLimitingInterface
}

// StartEndpointsImporter func
func StartEndpointsImporter(ctx context.Context, kubeClient kubeclient.Client, labelSelector string, annotationSources, annotationProbes string, resync time.Duration) (controller EndpointsImporter, err error) {
	if labelSelector == "" {
		return nil, fmt.Errorf("labelSelector required")
	}
	if annotationProbes == "" {
		return nil, fmt.Errorf("annotationProbes required")
	}
	logger := log.New(os.Stderr, "[importer] ", log.Flags())
	c := &endpointsImporter{
		ctx:               ctx,
		statusUpdater:     prober.NewStatusUpdater(ctx, logger),
		kubeClient:        kubeClient,
		labelSelector:     labelSelector,
		annotationSources: annotationSources,
		annotationProbes:  annotationProbes,
		logger:            logger,
		targets:           map[objectKey]*targetRecord{},
		updateQueue:       workqueue.NewRateLimitingQueue(informer.DefaultRateLimiter(5*time.Millisecond, 1000*time.Second, math.MaxFloat64, math.MaxInt32)),
	}
	c.informer = informer.NewInformer(kubeClient, informer.Opts{
		Logger:     logger,
		MaxRetries: -1,
		Handler: func(ctx context.Context, event informer.EventType, obj *unstructured.Unstructured, numRetries int) error {
			return c.handleEvent(ctx, event, obj)
		},
	})
	if c.client, c.resource, err = c.kubeClient.DynamicClient("v1", "Endpoints"); err != nil {
		return nil, err
	}
	c.informer.Watch("v1", "Endpoints", kubeClient.Namespace(), labelSelector, "", 1800*time.Second)
	go wait.Until(func() {
		for c.processUpdates(ctx) {
		}
	}, time.Second, ctx.Done())
	return c, c.informer.Run(ctx)
}

func toEndpoints(obj *unstructured.Unstructured) (*corev1.Endpoints, error) {
	ep := &corev1.Endpoints{}
	return ep, runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), ep)
}

func (c *endpointsImporter) handleEvent(ctx context.Context, event informer.EventType, obj *unstructured.Unstructured) error {
	endpoints, err := toEndpoints(obj)
	if err != nil {
		return err
	}
	switch event {
	case informer.EventAdd, informer.EventUpdate:
		probeConfs, sourceConfs, annotationProbes, annotationSources := []fluconf.Config{}, []fluconf.Config{},
			obj.GetAnnotations()[c.annotationProbes], obj.GetAnnotations()[c.annotationSources]
		if annotationProbes != "" {
			probeConfs = fluconf.Parse(annotationProbes, "probe", fluconf.Config{
				"interval": "5s",
				"timeout":  "5s",
				"fall":     "3",
				"rise":     "3",
			})
		}
		if annotationSources != "" {
			sourceConfs = fluconf.Parse(annotationSources, "source", fluconf.Config{
				"interval": "30s",
				"timeout":  "30s",
			})
		}
		return c.updateTarget(endpoints, probeConfs, sourceConfs)
	case informer.EventDelete:
		return c.updateTarget(endpoints, []fluconf.Config{}, []fluconf.Config{})
	}
	return nil
}
