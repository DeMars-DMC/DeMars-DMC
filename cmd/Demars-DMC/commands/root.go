package commands

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	cfg "github.com/Demars-DMC/Demars-DMC/config"
	"github.com/Demars-DMC/Demars-DMC/libs/cli"
	tmflags "github.com/Demars-DMC/Demars-DMC/libs/cli/flags"
	"github.com/Demars-DMC/Demars-DMC/libs/log"
)

var (
	config = cfg.DefaultConfig()
	logger = log.NewTMLogger(log.NewSyncWriter(os.Stdout))
)

func init() {
	registerFlagsRootCmd(RootCmd)
}

func registerFlagsRootCmd(cmd *cobra.Command) {
	cmd.PersistentFlags().String("log_level", config.LogLevel, "Log level")
}

// ParseConfig retrieves the default environment configuration,
// sets up the Demars-DMC root and ensures that the root exists
func ParseConfig() (*cfg.Config, error) {
	conf := cfg.DefaultConfig()
	err := viper.Unmarshal(conf)
	if err != nil {
		return nil, err
	}
	conf.SetRoot(conf.RootDir)
	cfg.EnsureRoot(conf.RootDir)
	return conf, err
}

// RootCmd is the root command for Demars-DMC core.
var RootCmd = &cobra.Command{
	Use:   "demars",
	Short: "Demars (BFT Consensus) in Go",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		if cmd.Name() == VersionCmd.Name() {
			return nil
		}
		config, err = ParseConfig()
		if err != nil {
			return err
		}
		logger, err = tmflags.ParseLogLevel(config.LogLevel, logger, cfg.DefaultLogLevel())
		if err != nil {
			return err
		}
		if viper.GetBool(cli.TraceFlag) {
			logger = log.NewTracingLogger(logger)
		}
		logger = logger.With("module", "main")
		return nil
	},
}
