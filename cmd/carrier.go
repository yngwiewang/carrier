package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"github.com/yngwiewang/carrier/pkg/config"
	"github.com/yngwiewang/carrier/pkg/host"
)

var (
	cfgFile        string
	hostsFileName  string
	authMode       string
	executeTimeout time.Duration
	logOutput      string
	dryRun         bool
	succeeded      string
	src            string
	dst            string
	mask           string
	cfg            *config.Config
	err            error
)

var rootCmd = &cobra.Command{
	Use:   "carrier",
	Short: "a command-line tool similar to Ansible ad-hoc mode, more efficient",
	Run:   func(cmd *cobra.Command, args []string) {},
}

// ExecuteC executes the command.
func Execute() {
	if err = rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var execCmd = &cobra.Command{
	Use:   "sh",
	Short: "execute shell command on remote hosts",
	RunE: func(cmd *cobra.Command, args []string) error {
		hosts, err := host.GetHosts(hostsFileName)
		if err != nil {
			return err
		}
		if len(args) == 0 {
			return errors.New("must specify a shell command to execute")
		}
		shellCmd := args[0]
		if dryRun {
			fmt.Printf("--------------------------------\n%s\n", shellCmd)
			return nil
		}
		hosts.ExecuteSSH(cfg, shellCmd)
		hosts.PrintResult()
		if err = hosts.SaveGob(); err != nil {
			return err
		}
		return nil
	},
}

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "print logs of the last execution",
	RunE: func(cmd *cobra.Command, args []string) error {
		hosts, err := host.LoadGob()
		if err != nil {
			return err
		}
		if logOutput == "table" {
			t := table.NewWriter()
			t.SetOutputMirror(os.Stdout)
			t.AppendHeader(table.Row{"ip", "succeeded", "result", "error", "duration"})
			for _, h := range hosts {
				if succeeded == "true" && !h.IsSucceeded {
					continue
				}
				if succeeded == "false" && h.IsSucceeded {
					continue
				}
				t.AppendRow([]interface{}{h.IP, h.IsSucceeded, h.Result, h.Error, h.ExecDuration})
				t.AppendSeparator()
			}
			t.Render()
		} else {
			fmt.Println("ip", "succeeded", "result", "error", "duration")
			for _, h := range hosts {
				if succeeded == "true" && !h.IsSucceeded {
					continue
				}
				if succeeded == "false" && h.IsSucceeded {
					continue
				}
				fmt.Printf("%s,%t,%s,%s,%v\n", h.IP, h.IsSucceeded, h.Result, h.Error, h.ExecDuration)
			}
		}
		return nil
	},
}

var hostsCmd = &cobra.Command{
	Use:   "hosts",
	Short: "print host list, could be filtered by the state of the last execution",
	RunE: func(cmd *cobra.Command, args []string) error {
		hosts, err := host.LoadGob()
		if err != nil {
			return err
		}
		for _, h := range hosts {
			if succeeded == "true" && !h.IsSucceeded {
				continue
			}
			if succeeded == "false" && h.IsSucceeded {
				continue
			}
			fmt.Printf("%s,%s,%s,%s\n", h.IP, h.Port, h.Username, h.Password)
		}
		return nil
	},
}

var copyCmd = &cobra.Command{
	Use:   "cp",
	Short: "copy file to remote hosts, like scp",
	RunE: func(cmd *cobra.Command, args []string) error {
		hosts, err := host.GetHosts(hostsFileName)
		if err != nil {
			return err
		}
		if err = hosts.ExecuteSCP(cfg, src, dst, mask); err != nil {
			return err
		}
		hosts.PrintResult()
		if err = hosts.SaveGob(); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "carrier.yml", "config file")
	rootCmd.PersistentFlags().StringVarP(&hostsFileName, "inventory", "i", "", "remote host list to read from (default value in config file)")
	rootCmd.PersistentFlags().StringVarP(&authMode, "auth-mode", "a", "", "remote hosts' ssh authentication mode, could be password or key(default value in config file)")
	execCmd.Flags().DurationVarP(&executeTimeout, "timeout", "t", 0*time.Second, "timeout to execute remote shell(default value in config file)")
	execCmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "print without executing the shell command, for checking")
	logsCmd.Flags().StringVarP(&logOutput, "output", "o", "table", "log output format(can be table or csv, default is table)")
	logsCmd.Flags().StringVarP(&succeeded, "succeeded", "s", "all", "is the execution successful(can be all, true or false, default is all)")
	hostsCmd.Flags().StringVarP(&succeeded, "succeeded", "s", "all", "is the execution successful(can be all, true or false, default is all)")
	copyCmd.Flags().StringVarP(&src, "src", "s", "", "source file on local host")
	copyCmd.Flags().StringVarP(&dst, "dst", "d", "", "destination file on remote hosts")
	copyCmd.Flags().StringVarP(&mask, "mask", "m", "0755", "mask code of destination file(default is 0755)")
	copyCmd.MarkFlagRequired("source")
	copyCmd.MarkFlagRequired("destination")

	cobra.OnInitialize(initConfig)
	rootCmd.AddCommand(execCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(hostsCmd)
	rootCmd.AddCommand(copyCmd)
}

func initConfig() {
	cfg, err = config.NewConfig(cfgFile)
	if err != nil {
		fmt.Printf("failed to parse config file, err: %v", err)
		os.Exit(1)
	}
	if hostsFileName == "" {
		hostsFileName = cfg.HostsFileName
	}
	if authMode != "" {
		cfg.AuthMode = authMode
	}
	if executeTimeout != 0*time.Second {
		cfg.ExecuteTimeout = executeTimeout
	}
}

func main() {
	Execute()
}
