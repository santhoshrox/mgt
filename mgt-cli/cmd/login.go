package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/santhosh/mgt/pkg/client"
	"github.com/santhosh/mgt/pkg/config"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with mgt-be via GitHub",
	Run: func(cmd *cobra.Command, args []string) {
		c := client.New()
		ds, err := c.DeviceStart()
		if err != nil {
			fmt.Fprintf(os.Stderr, "mgt: could not start login: %v\n", err)
			os.Exit(1)
		}

		fmt.Fprintf(os.Stderr, "Open this URL in your browser to continue:\n  %s\n", ds.VerificationURL)
		fmt.Fprintf(os.Stderr, "Verify the device code matches: %s\n\n", ds.UserCode)
		_ = openBrowser(ds.VerificationURL)
		fmt.Fprint(os.Stderr, "Waiting for confirmation")

		// Poll for up to ~5 min.
		deadline := time.Now().Add(5 * time.Minute)
		for time.Now().Before(deadline) {
			token, err := c.DevicePoll(ds.State)
			if err == nil && token != "" {
				if err := config.SetToken(token); err != nil {
					fmt.Fprintf(os.Stderr, "\nmgt: could not save token: %v\n", err)
					os.Exit(1)
				}
				c.Token = token
				me, err := c.Me()
				if err != nil {
					fmt.Fprintf(os.Stderr, "\nmgt: token saved but /me failed: %v\n", err)
					os.Exit(1)
				}
				fmt.Fprintf(os.Stderr, "\nLogged in as %s.\n", me.Login)
				return
			}
			fmt.Fprint(os.Stderr, ".")
			time.Sleep(2 * time.Second)
		}
		fmt.Fprintln(os.Stderr, "\nmgt: login timed out")
		os.Exit(1)
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Forget the local mgt-be token",
	Run: func(cmd *cobra.Command, args []string) {
		if err := config.SetToken(""); err != nil {
			fmt.Fprintf(os.Stderr, "mgt: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Logged out.")
	},
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}
	return cmd.Start()
}

func init() {
	rootCmd.AddCommand(loginCmd, logoutCmd)
}
