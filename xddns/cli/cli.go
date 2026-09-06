package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"iter"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/iceflowre/xddns/xddns"
	"github.com/iceflowre/xddns/xddns/config"
	"github.com/iceflowre/xddns/xddns/lib"
	"github.com/iceflowre/xddns/xddns/notifier"
	"github.com/iceflowre/xddns/xddns/provider"
	"github.com/iceflowre/xddns/xddns/registry"
	"github.com/iceflowre/xddns/xddns/resolver"
)

type cliCmd struct {
	cobra.Command

	configPath string
}

// NewCli creates a new CLI command.
func NewCli(ctx context.Context) *cobra.Command {
	cmd := &cliCmd{
		Use:   "xddns",
		Short: "xddns is a dynamic DNS updater",
	}

	cmd.SetContext(ctx)
	cmd.PersistentFlags().StringVarP(&cmd.configPath, "config", "c", "", "path to config file")

	cmd.AddCommand(newConfigCmd(cmd))
	cmd.AddCommand(newDaemonCmd(cmd))
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newUpdateCmd(cmd))
	cmd.AddCommand(newVersionCmd())

	return &cmd.Command
}

type configOpts struct {
	cli *cliCmd

	showSecrets bool
}

func newConfigCmd(c *cliCmd) *cobra.Command {
	opts := &configOpts{cli: c}

	cfgCmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration management commands",
	}

	{
		cmd := &cobra.Command{
			Use:           "validate",
			Short:         "Validate the configuration for errors",
			SilenceUsage:  true,
			SilenceErrors: true,
			RunE:          opts.configValidate,
		}
		cfgCmd.AddCommand(cmd)
	}
	{
		cmd := &cobra.Command{
			Use:          "path",
			Short:        "Print the path to the configuration file",
			SilenceUsage: true,
			RunE:         opts.configPath,
		}
		cfgCmd.AddCommand(cmd)
	}
	{
		cmd := &cobra.Command{
			Use:          "show",
			Short:        "Show the resolved active configuration",
			SilenceUsage: true,
			RunE:         opts.configShow,
		}
		cmd.Flags().BoolVar(&opts.showSecrets, "show-secrets", false, "show secrets in the output (use with caution)")

		cfgCmd.AddCommand(cmd)
	}

	return cfgCmd
}

func (opts *configOpts) configValidate(cmd *cobra.Command, _args []string) error {
	cfg, _, err := config.LoadConfig(opts.cli.configPath)
	if err != nil {
		cmd.Println(err.Error())

		return err
	}

	err = xddns.ValidateConfig(cfg)
	if err != nil {
		cmd.Println(err.Error())
	}

	return err
}

func (opts *configOpts) configPath(cmd *cobra.Command, _args []string) error {
	cfgPath := opts.cli.configPath
	if cfgPath == "" {
		cfgPath = config.FindConfigPath()
	}
	if cfgPath == "" {
		return config.ErrNoConfigFound
	}
	cmd.Println(cfgPath)

	return nil
}

func (opts *configOpts) configShow(cmd *cobra.Command, _args []string) error {
	cfg, cfgPath, err := config.LoadConfig(opts.cli.configPath)
	if err != nil {
		return err
	}

	resolvedCfg, errs := xddns.ResolveConfig(*cfg)
	if len(errs) > 0 && resolvedCfg == nil {
		return errors.Join(err)
	}
	ctx := context.Background()
	if opts.showSecrets {
		ctx = lib.WithUnredactedSecrets(ctx)
	}

	data, err := yaml.MarshalContext(ctx, resolvedCfg)
	if err != nil {
		return err
	}

	if len(errs) > 0 {
		cmd.Printf("# This resolved configuration is incomplete and invalid.\n# Run `xddns config validate --config %s` for details.\n", cfgPath)
		cmd.Println()
	}

	cmd.Println(string(data))

	return errors.Join(err)
}

type daemonOpts struct {
	cli *cliCmd

	logLevel string
}

func newDaemonCmd(c *cliCmd) *cobra.Command {
	opts := &daemonOpts{cli: c}

	cmd := &cobra.Command{
		Use:          "daemon",
		Short:        "Start the updater in daemon mode, running periodic checks and updates",
		SilenceUsage: true,
		RunE:         opts.runE,
	}

	cmd.Flags().StringVarP(&opts.logLevel, "log-level", "l", "", "set the log level (debug, info, warn, error)")

	return cmd
}

func (opts *daemonOpts) runE(cmd *cobra.Command, _args []string) error {
	app, err := createApp(opts.cli.configPath, xddns.WithLogLevel(opts.logLevel))
	if err != nil {
		return err
	}

	return app.RunDaemon(cmd.Context())
}

type listOpts struct {
	details bool
}

func newListCmd() *cobra.Command {
	opts := &listOpts{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all registered providers, resolvers, and notifiers",
		Run:   opts.run,
	}

	cmd.Flags().BoolVar(&opts.details, "details", false, "show detailed information about each item")

	return cmd
}

func (opts *listOpts) run(cmd *cobra.Command, _args []string) {
	out := cmd.OutOrStdout()

	printSection(out, "Resolver", resolver.List(), opts.details)
	printSection(out, "Provider", provider.List(), opts.details)
	printSection(out, "Notifier", notifier.List(), opts.details)
}

func printSection[T any](out io.Writer, header string, entries iter.Seq2[string, registry.Entry[T]], details bool) { //nolint:revive
	_, _ = out.Write([]byte(header + ":\n"))
	if details {
		printEntriesWithDetails(entries, out)
	} else {
		for name := range entries {
			writeEntryName(out, name)
		}
	}
}

// printEntriesWithDetails prints each entry's name followed by its indented YAML config.
func printEntriesWithDetails[T any](entries iter.Seq2[string, registry.Entry[T]], out io.Writer) {
	for name, entry := range entries {
		writeEntryName(out, name)

		yamlData, err := marshalYAMLWithComments(entry.Config())
		if err != nil {
			_, _ = out.Write([]byte("  YAML config not available\n"))

			continue
		}
		yamlData = bytes.ReplaceAll(yamlData, []byte("\n"), []byte("\n  "))
		_, _ = out.Write([]byte("  "))
		_, _ = out.Write(yamlData)
		_, _ = out.Write([]byte("\n"))
	}
}

func writeEntryName(out io.Writer, name string) {
	_, _ = out.Write([]byte("- "))
	_, _ = out.Write([]byte(name))
	_, _ = out.Write([]byte("\n"))
}

type updateOpts struct {
	cli *cliCmd

	dryRun       bool
	dryRunNotify bool
	logLevel     string
}

func newUpdateCmd(c *cliCmd) *cobra.Command {
	opts := &updateOpts{cli: c}

	cmd := &cobra.Command{
		Use:          "update",
		Short:        "Run a single update check and exit immediately",
		SilenceUsage: true,
		RunE:         opts.runE,
	}

	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "simulate update without making actual API changes")
	cmd.Flags().BoolVar(&opts.dryRunNotify, "dry-run-notify", false, "push notifications in a dry run")
	cmd.Flags().StringVarP(&opts.logLevel, "log-level", "l", "", "set the log level (debug, info, warn, error)")
	requireFlag(cmd, "dry-run-notify", "dry-run")

	return cmd
}

func (opts *updateOpts) runE(cmd *cobra.Command, _args []string) error {
	app, err := createApp(opts.cli.configPath, xddns.WithLogLevel(opts.logLevel))
	if err != nil {
		return err
	}

	return app.Update(
		cmd.Context(),
		xddns.WithDryRun(opts.dryRun),
		xddns.WithDryRunNotify(opts.dryRunNotify),
	)
}

func newVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version of xddns",
		Run: func(cmd *cobra.Command, _args []string) {
			cmd.Println(xddns.Version())
		},
	}

	return cmd
}

func createApp(path string, opts ...xddns.AppOption) (app *xddns.App, err error) {
	cfg, _, err := config.LoadConfig(path)
	if err != nil {
		return nil, err
	}

	return xddns.NewApp(*cfg, opts...)
}

// requireFlag enforces that flag can only be used if requiredFlag is also set.
func requireFlag(cmd *cobra.Command, flag string, requiredFlag string) {
	prevHook := cmd.PreRunE
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed(flag) && !cmd.Flags().Changed(requiredFlag) {
			cmd.SilenceUsage = false

			return fmt.Errorf("--%s requires --%s", flag, requiredFlag) //nolint:err113
		}
		if prevHook != nil {
			return prevHook(cmd, args)
		}

		return nil
	}
}
