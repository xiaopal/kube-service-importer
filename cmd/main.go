package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/xiaopal/kube-informer/pkg/appctx"
	"github.com/xiaopal/kube-informer/pkg/kubeclient"
)

var (
	application   appctx.Interface
	globalOptions = &struct {
		Logger      *log.Logger
		BindAddr    string
		APIClient   string
		APISecret   string
		APIEndpoint string
		KubeClient  kubeclient.Client
	}{}
)

func newLogger(module string) *log.Logger {
	return log.New(os.Stderr, fmt.Sprintf("[%s] ", module), log.Flags())
}

func runApplication(args []string) error {
	application = appctx.Start()
	defer application.End()
	<-application.Context().Done()
	return nil
}

func main() {
	globalOptions.Logger = newLogger("main")
	globalOptions.KubeClient = kubeclient.NewClient(&kubeclient.ClientOpts{})
	cmd := &cobra.Command{
		Use: fmt.Sprintf("%s [flags]", os.Args[0]),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApplication(args)
		},
	}
	flags := cmd.Flags()
	flags.AddGoFlagSet(flag.CommandLine)
	globalOptions.KubeClient.BindFlags(flags, "IMPORTER_OPTS_")

	if err := cmd.Execute(); err != nil {
		globalOptions.Logger.Fatal(err)
	}
}
