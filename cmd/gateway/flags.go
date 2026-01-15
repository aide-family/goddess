package gateway

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/aide-family/goddess/cmd"
)

var flags Flags

type Flags struct {
	*cmd.GlobalFlags
	ctrlName          string
	ctrlService       string
	proxyAddrs        []string
	proxyConfig       string
	priorityConfigDir string
	withDebug         bool
}

func (f *Flags) addFlags(c *cobra.Command) {
	f.GlobalFlags = cmd.GetGlobalFlags()
	c.PersistentFlags().StringVar(&f.ctrlName, "ctrl.name", os.Getenv("ADVERTISE_NAME"), "control gateway name, eg: gateway")
	c.PersistentFlags().StringVar(&f.ctrlService, "ctrl.service", "", "control service host, eg: http://127.0.0.1:8000")
	c.PersistentFlags().StringVar(&f.proxyConfig, "conf", "./cmd/gateway/config.yaml", "config path, eg: -conf config.yaml")
	c.PersistentFlags().StringVar(&f.priorityConfigDir, "conf.priority", "", "priority config directory, eg: -conf.priority ./canary")
	c.PersistentFlags().BoolVar(&f.withDebug, "debug", false, "enable debug handlers")
	c.PersistentFlags().StringSliceVar(&f.proxyAddrs, "addr", []string{"0.0.0.0:8080"}, "proxy address, eg: -addr 0.0.0.0:8080")
}
