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
	Short: "Set a config value; omit value to clear",
	Long:  "Supported keys: branch_prefix (user-level ~/.mgt), trunk, remote (repo-level <git-root>/.mgt)",
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
		case "trunk", "remote":
			if err := config.SetRepoValue(key, value); err != nil {
				fmt.Fprintf(os.Stderr, "mgt: %v\n", err)
				os.Exit(1)
			}
			if value == "" {
				fmt.Printf("%s cleared\n", key)
			} else {
				fmt.Printf("%s set to %q\n", key, value)
			}
		default:
			fmt.Fprintf(os.Stderr, "mgt: unknown key %q (supported: branch_prefix, trunk, remote)\n", key)
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
		var v string
		switch key {
		case "branch_prefix":
			v = config.BranchPrefix()
		case "trunk":
			v = config.Trunk()
		case "remote":
			v = config.Remote()
		default:
			fmt.Fprintf(os.Stderr, "mgt: unknown key %q (supported: branch_prefix, trunk, remote)\n", key)
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
	userPath := config.ConfigPath()
	if userPath == "" {
		return fmt.Errorf("could not determine config path (home dir)")
	}

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

	// User-level: branch prefix
	value, err := prompt("Branch prefix for new stacks (e.g. santhosh/, leave empty for none): ")
	if err != nil {
		return err
	}
	if err := config.SetBranchPrefix(value); err != nil {
		return err
	}
	if value == "" {
		fmt.Fprintf(os.Stderr, "  branch_prefix: (none)\n")
	} else {
		fmt.Fprintf(os.Stderr, "  branch_prefix: %s\n", value)
	}

	// Repo-level: trunk
	repoPath := config.RepoConfigPath()
	if repoPath == "" {
		fmt.Fprintf(os.Stderr, "Not in a git repo — skipping repo config (trunk, remote)\n")
		return nil
	}

	trunk, err := prompt("Trunk branch name (default: main): ")
	if err != nil {
		return err
	}
	if trunk == "" {
		trunk = "main"
	}
	if err := config.SetRepoValue("trunk", trunk); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "  trunk: %s\n", trunk)

	// Repo-level: remote
	remote, err := prompt("Default remote (default: origin): ")
	if err != nil {
		return err
	}
	if remote == "" {
		remote = "origin"
	}
	if err := config.SetRepoValue("remote", remote); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "  remote: %s\n", remote)

	fmt.Fprintf(os.Stderr, "Wrote %s and %s\n", userPath, repoPath)
	return nil
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configInitCmd)
}
