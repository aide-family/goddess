package control

import (
	"github.com/spf13/cobra"

	"github.com/aide-family/goddess/cmd"
)

var flags Flags

type Flags struct {
	*cmd.GlobalFlags
	httpAddr string
	dataDir  string
}

func (f *Flags) addFlags(c *cobra.Command) {
	f.GlobalFlags = cmd.GetGlobalFlags()
	c.PersistentFlags().StringVar(&f.httpAddr, "http.addr", ":8000", "HTTP server address, eg: 0.0.0.0:8000")
	c.PersistentFlags().StringVar(&f.dataDir, "data.dir", "./data/control", "Data directory for storing gateway configs")
}
