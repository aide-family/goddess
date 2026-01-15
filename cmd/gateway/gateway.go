// Package gateway is the main package for the gateway service.
package gateway

import (
	"context"
	"net/http"

	_ "net/http/pprof"

	_ "github.com/aide-family/goddess/discovery/consul"
	_ "github.com/aide-family/goddess/discovery/etcd"
	_ "github.com/aide-family/goddess/middleware/bbr"
	_ "github.com/aide-family/goddess/middleware/cors"
	_ "github.com/aide-family/goddess/middleware/jwt"
	_ "github.com/aide-family/goddess/middleware/logging"
	_ "github.com/aide-family/goddess/middleware/namespace"
	_ "github.com/aide-family/goddess/middleware/rewrite"
	_ "github.com/aide-family/goddess/middleware/streamrecorder"
	_ "github.com/aide-family/goddess/middleware/tracing"
	_ "github.com/aide-family/goddess/middleware/transcoder"
	_ "go.uber.org/automaxprocs"

	"github.com/aide-family/magicbox/hello"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/spf13/cobra"

	"github.com/aide-family/goddess/client"
	"github.com/aide-family/goddess/cmd"
	"github.com/aide-family/goddess/config"
	configLoader "github.com/aide-family/goddess/config/config-loader"
	"github.com/aide-family/goddess/discovery"
	"github.com/aide-family/goddess/middleware"
	"github.com/aide-family/goddess/middleware/circuitbreaker"
	"github.com/aide-family/goddess/proxy"
	"github.com/aide-family/goddess/proxy/debug"
	"github.com/aide-family/goddess/server"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gateway",
		Short: "goddess gateway service",
		Long:  "goddess gateway service",
		Run:   run,
	}
	flags.addFlags(cmd)
	return cmd
}

func run(_ *cobra.Command, _ []string) {
	ctx := context.Background()
	var ctrlLoader *configLoader.CtrlConfigLoader
	if flags.ctrlService != "" {
		log.Infof("setup control service to: %q", flags.ctrlService)
		ctrlLoader = configLoader.New(flags.ctrlName, flags.ctrlService, flags.proxyConfig, flags.priorityConfigDir)
		if err := ctrlLoader.Load(ctx); err != nil {
			log.Errorf("failed to do initial load from control service: %v, using local config instead", err)
		}
		if err := ctrlLoader.LoadFeatures(ctx); err != nil {
			log.Errorf("failed to do initial feature load from control service: %v, using default value instead", err)
		}
		go ctrlLoader.Run(ctx)
	}

	confLoader, err := config.NewFileLoader(flags.proxyConfig, flags.priorityConfigDir)
	if err != nil {
		log.Fatalf("failed to create config file loader: %v", err)
	}
	defer confLoader.Close()
	bc, err := confLoader.Load(context.Background())
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	discovery, err := discovery.Create(bc.Discovery)
	if err != nil {
		log.Fatalf("failed to create discovery: %v, using default discovery instead", err)
	}
	clientFactory := client.NewFactory(discovery)
	p, err := proxy.New(clientFactory, middleware.Create)
	if err != nil {
		log.Fatalf("failed to new proxy: %v", err)
	}

	buildContext := client.NewBuildContext(bc)
	circuitbreaker.Init(buildContext, clientFactory)
	if err := p.Update(buildContext, bc); err != nil {
		log.Fatalf("failed to update service config: %v", err)
	}
	reloader := func() error {
		bc, err := confLoader.Load(context.Background())
		if err != nil {
			log.Errorf("failed to load config: %v", err)
			return err
		}
		buildContext := client.NewBuildContext(bc)
		circuitbreaker.SetBuildContext(buildContext)
		if err := p.Update(buildContext, bc); err != nil {
			log.Errorf("failed to update service config: %v", err)
			return err
		}
		log.Infof("config reloaded")
		return nil
	}
	confLoader.Watch(reloader)

	var serverHandler http.Handler = p
	if flags.withDebug {
		debug.Register("proxy", p)
		debug.Register("config", confLoader)
		if ctrlLoader != nil {
			debug.Register("ctrl", ctrlLoader)
		}
		serverHandler = debug.MashupWithDebugHandler(p)
	}
	servers := make([]transport.Server, 0, len(flags.proxyAddrs))
	for _, addr := range flags.proxyAddrs {
		servers = append(servers, server.NewProxy(serverHandler, addr))
	}
	app := kratos.New(
		kratos.Name(bc.Name),
		kratos.Context(ctx),
		kratos.Server(
			servers...,
		),
	)
	globalFlags := cmd.GetGlobalFlags()
	envOpts := []hello.Option{
		hello.WithVersion(globalFlags.Version),
		hello.WithID(globalFlags.Hostname),
		hello.WithEnv("PROD"),
		hello.WithMetadata(map[string]string{}),
		hello.WithName(globalFlags.Name),
	}
	hello.SetEnvWithOption(envOpts...)
	hello.Hello()
	if err := app.Run(); err != nil {
		log.Errorf("failed to run servers: %v", err)
	}
}
