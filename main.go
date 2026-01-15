package main

import (
	_ "embed"
	"os"

	"github.com/spf13/cobra"

	"github.com/aide-family/magicbox/log"
	"github.com/aide-family/magicbox/log/stdio"
	klog "github.com/go-kratos/kratos/v2/log"

	"github.com/aide-family/goddess/cmd"
	"github.com/aide-family/goddess/cmd/gateway"
	"github.com/aide-family/goddess/cmd/version"
	"github.com/aide-family/goddess/pkg/merr"
)

var (
	Name        = "goddess"
	Version     = "latest"
	BuildTime   = "now"
	Author      = "Aide Family"
	Email       = "aidecloud@163.com"
	Repo        = "https://github.com/aide-family/goddess"
	hostname, _ = os.Hostname()
)

//go:embed description.txt
var Description string

func main() {
	cmd.SetGlobalFlags(
		cmd.WithGlobalFlagsName(Name),
		cmd.WithGlobalFlagsHostname(hostname),
		cmd.WithGlobalFlagsVersion(Version),
		cmd.WithGlobalFlagsBuildTime(BuildTime),
		cmd.WithGlobalFlagsAuthor(Author),
		cmd.WithGlobalFlagsEmail(Email),
		cmd.WithGlobalFlagsREPO(Repo),
		cmd.WithGlobalFlagsDescription(Description),
	)

	children := []*cobra.Command{
		version.NewCmd(),
		gateway.NewCmd(),
	}
	cmd.Execute(cmd.NewCmd(), children...)
}

func init() {
	logger, err := log.NewLogger(stdio.LoggerDriver())
	if err != nil {
		panic(merr.ErrorInternal("new logger failed with error: %v", err).WithCause(err))
	}
	logger = klog.With(logger,
		"ts", klog.DefaultTimestamp,
	)
	filterLogger := klog.NewFilter(logger, klog.FilterLevel(klog.LevelInfo))
	helper := klog.NewHelper(filterLogger)
	klog.SetLogger(helper.Logger())
}
