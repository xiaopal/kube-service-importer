package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/xiaopal/kube-informer/pkg/appctx"
	"github.com/xiaopal/kube-informer/pkg/kubeclient"
	"github.com/xiaopal/kube-informer/pkg/leaderelect"
	"github.com/xiaopal/kube-informer/pkg/subreaper"

	"github.com/xiaopal/kube-service-importer/pkg/controller"
)

var (
	application   appctx.Interface
	globalOptions = &struct {
		Importer       string
		Logger         *log.Logger
		Prefix         string
		KubeClient     kubeclient.Client
		LeaderHelper   leaderelect.Helper
		ResyncDuration time.Duration
		ListenAddr     string
	}{}
)

func newLogger(module string) *log.Logger {
	return log.New(os.Stderr, fmt.Sprintf("[%s] ", module), log.Flags())
}

func runApplication(args []string) error {
	application = appctx.Start()
	defer application.End()

	if os.Getpid() == 1 {
		subreaper.Start(application.Context())
	}
	globalOptions.LeaderHelper.Run(application.Context(), func(ctx context.Context) {
		labelSelector, annotationSources, annotationProbes := fmt.Sprintf("%s%s=%s", globalOptions.Prefix, "importer", globalOptions.Importer),
			fmt.Sprintf("%s%s", globalOptions.Prefix, "sources"),
			fmt.Sprintf("%s%s", globalOptions.Prefix, "probes")
		controller.StartEndpointsImporter(ctx, globalOptions.KubeClient, labelSelector, annotationSources, annotationProbes, globalOptions.ResyncDuration, globalOptions.ListenAddr)
	})
	<-application.Context().Done()
	return nil
}

func main() {
	logger, kubeClient := newLogger("main"), kubeclient.NewClient(&kubeclient.ClientOpts{})
	cmd := &cobra.Command{
		Use: fmt.Sprintf("%s [flags]", os.Args[0]),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApplication(args)
		},
	}
	leaderHelper := leaderelect.NewHelper(&leaderelect.HelperOpts{
		DefaultNamespaceFunc: kubeClient.DefaultNamespace,
		GetConfigFunc:        kubeClient.GetConfig,
	})
	flags := cmd.Flags()
	flags.AddGoFlagSet(flag.CommandLine)
	kubeClient.BindFlags(flags, "IMPORTER_OPTS_")
	leaderHelper.BindFlags(flags, "IMPORTER_OPTS_")
	flags.StringVar(&globalOptions.Importer, "importer", "", "importer profile(watch label value)")
	flags.StringVarP(&globalOptions.Prefix, "prefix", "p", "kube-service-importer.xiaopal.github.com/", "watch label/annotations prefix")
	flags.DurationVar(&globalOptions.ResyncDuration, "resync", 0, "resync period")
	flags.StringVar(&globalOptions.ListenAddr, "listen", "", "start http server to handle /health and /endpoints, eg. :8080")
	globalOptions.Logger, globalOptions.KubeClient, globalOptions.LeaderHelper = logger, kubeClient, leaderHelper
	if err := cmd.Execute(); err != nil {
		logger.Fatal(err)
	}
}
