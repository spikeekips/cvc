package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/spikeekips/cvc"
)

var (
	cmd *cobra.Command
)

type LogLevel struct {
	Name string
}

type Config struct {
	cvc.BaseGroup

	Verbose   bool   `flag-help:"verbose"`
	SetString string `flag-help:"set integer"`
	SetInt    int    `flag-help:"set string"`

	Log *LogConfig
}

func (c *Config) ParseEnvSetInt(s string) (int, error) {
	i, err := strconv.ParseInt(s, 10, 64)
	return int(i), err
}

type LogConfig struct {
	cvc.BaseGroup

	File   string   `flag-help:"log output file"`
	Level  LogLevel `flag-help:"log format {terminal json}"`
	Format string   `flag-help:"log level {debug error warn crit}"`
}

func (l *LogConfig) ParseLevel(input string) (LogLevel, error) {
	switch input {
	case "debug", "error", "warn", "crit":
	default:
		return LogLevel{}, fmt.Errorf("unknown log level")
	}
	return LogLevel{Name: input}, nil
}

func init() {
	var vp *viper.Viper
	var manager *cvc.Manager

	cmd = &cobra.Command{
		Use:   "naru",
		Short: "naru is API server for SEBAK",
		PreRun: func(cmd *cobra.Command, args []string) {
			_, err := manager.Merge()
			if err == nil {
				return
			}

			fmt.Println("Error:", err.Error())
			cmd.Help()
			os.Exit(1)
		},
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("")
			fmt.Println("# loaded config:")
			b, _ := json.MarshalIndent(manager.Config(), "", "  ")
			fmt.Println(string(b))
		},
	}

	vp = viper.New()
	config := &Config{
		SetString: "find me",
		SetInt:    100,
		Log: &LogConfig{
			File:   "naru.log",
			Level:  LogLevel{Name: "debug"},
			Format: "terminal",
		},
	}
	manager = cvc.NewManager("naru", config, cmd, vp)
}

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
