package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/santhoshrox/mgt/pkg/config"
	"github.com/spf13/cobra"
)

const supportedConfigKeys = "server_url, server_grpc_addr, branch_prefix"

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Get or set mgt configuration (~/.mgt/config)",
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "Set a config value; omit value to clear",
	Long:  "Supported keys: " + supportedConfigKeys,
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		val := ""
		if len(args) >= 2 {
			val = args[1]
		}
		switch key {
		case "server_url":
			must(config.SetServerURL(val))
		case "server_grpc_addr":
			must(config.SetGRPCAddr(val))
		case "branch_prefix":
			must(config.SetBranchPrefix(val))
		default:
			fmt.Fprintf(os.Stderr, "mgt: unknown key %q (supported: %s)\n", key, supportedConfigKeys)
			os.Exit(1)
		}
		if val == "" {
			fmt.Printf("%s cleared\n", key)
		} else {
			fmt.Printf("%s set to %q\n", key, val)
		}
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Print a config value",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var v string
		switch args[0] {
		case "server_url":
			v = config.ServerURL()
		case "server_grpc_addr":
			v = config.GRPCAddr()
		case "branch_prefix":
			v = config.BranchPrefix()
		default:
			fmt.Fprintf(os.Stderr, "mgt: unknown key %q (supported: %s)\n", args[0], supportedConfigKeys)
			os.Exit(1)
		}
		if v == "" {
			fmt.Println("(none)")
		} else {
			fmt.Println(v)
		}
	},
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Set config interactively (reads from stdin)",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		ask := func(msg, def string) string {
			fmt.Fprintf(os.Stderr, "%s", msg)
			s := bufio.NewScanner(os.Stdin)
			if !s.Scan() {
				return def
			}
			v := strings.TrimSpace(s.Text())
			if v == "" {
				return def
			}
			return v
		}

		current := config.ServerURL()
		url := ask(fmt.Sprintf("Server URL [%s]: ", current), current)
		must(config.SetServerURL(url))

		grpcAddr := ask(fmt.Sprintf("Server gRPC addr [%s]: ", config.GRPCAddr()), config.GRPCAddr())
		must(config.SetGRPCAddr(grpcAddr))

		prefix := ask(fmt.Sprintf("Branch prefix [%s]: ", config.BranchPrefix()), "")
		must(config.SetBranchPrefix(prefix))

		fmt.Fprintf(os.Stderr, "Wrote %s\n", config.ConfigPath())
	},
}

func must(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "mgt: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	configCmd.AddCommand(configSetCmd, configGetCmd, configInitCmd)
	rootCmd.AddCommand(configCmd)
}
