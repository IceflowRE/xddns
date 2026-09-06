// xddns executable.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/iceflowre/xddns/xddns/cli"
	_ "github.com/iceflowre/xddns/xddns/notifier/all"
	_ "github.com/iceflowre/xddns/xddns/provider/all"
	_ "github.com/iceflowre/xddns/xddns/resolver/all"
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
		syscall.SIGHUP,
		syscall.SIGQUIT,
	)
	defer stop()

	err := cli.NewCli(ctx).Execute()
	if err != nil {
		stop()
		os.Exit(1) //nolint:gocritic
	}
}
