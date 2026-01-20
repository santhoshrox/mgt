package cmd

import (
	"github.com/santhosh/mgt/pkg/gt"
	"github.com/spf13/cobra"
)

var stackSubmitCmd = &cobra.Command{
	Use:                "stack-submit",
	Short:              "Submit the entire current stack as pull requests",
	DisableFlagParsing: true,
	Args:               cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		gtArgs := []string{"stack", "submit"}
		gtArgs = append(gtArgs, args...)
		gt.Exec(gtArgs...)
	},
}

func init() {
	rootCmd.AddCommand(stackSubmitCmd)
}
