package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/santhosh/mgt/pkg/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Get or set mgt configuration (~/.mgt)",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "Set a config value; omit value to clear (e.g. no prefix)",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		var value string
		if len(args) >= 2 {
			value = args[1]
		}
		switch key {
		case "branch_prefix":
			if err := config.SetBranchPrefix(value); err != nil {
				fmt.Fprintf(os.Stderr, "mgt: %v\n", err)
				os.Exit(1)
			}
			if value == "" {
				fmt.Println("branch_prefix cleared (no prefix)")
			} else {
				fmt.Printf("branch_prefix set to %q\n", value)
			}
		default:
			fmt.Fprintf(os.Stderr, "mgt: unknown key %q (supported: branch_prefix)\n", key)
			os.Exit(1)
		}
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Print a config value",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		switch key {
		case "branch_prefix":
			v := config.BranchPrefix()
			if v == "" {
				fmt.Println("(none)")
			} else {
				fmt.Println(v)
			}
		default:
			fmt.Fprintf(os.Stderr, "mgt: unknown key %q (supported: branch_prefix)\n", key)
			os.Exit(1)
		}
	},
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Set config by answering prompts (reads from stdin)",
	Long:  "Asks questions and reads answers from stdin. Works interactively or piped: echo 'santhosh' | mgt config init",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runConfigInit(); err != nil {
			fmt.Fprintf(os.Stderr, "mgt: %v\n", err)
			os.Exit(1)
		}
	},
}

func runConfigInit() error {
	path := config.ConfigPath()
	if path == "" {
		return fmt.Errorf("could not determine config path (home dir)")
	}

	// Prompt to stderr so stdin can be used for piped input
	prompt := func(msg string) (string, error) {
		fmt.Fprint(os.Stderr, msg)
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return "", err
			}
			return "", nil
		}
		return strings.TrimSpace(scanner.Text()), nil
	}

	// Branch prefix
	msg := "Branch prefix for new stacks (e.g. santhosh/, leave empty for none): "
	value, err := prompt(msg)
	if err != nil {
		return err
	}

	if err := config.SetBranchPrefix(value); err != nil {
		return err
	}

	if value == "" {
		fmt.Fprintf(os.Stderr, "Wrote %s (no branch prefix)\n", path)
	} else {
		fmt.Fprintf(os.Stderr, "Wrote %s with branch_prefix=%s\n", path, value)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configInitCmd)
}
