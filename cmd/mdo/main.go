package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/nickstrad/mini-durable-objects/internal/apiclient"
	"github.com/spf13/cobra"
)

func main() {
	cmd := newRootCommand(&http.Client{Timeout: 10 * time.Second})
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand(httpClient *http.Client) *cobra.Command {
	var server string
	root := &cobra.Command{
		Use:           "mdo",
		Short:         "Create, read, claim, and finish durable object tasks",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&server, "server", "http://127.0.0.1:8787", "API base URL")

	run := func(name string, call func(*cobra.Command, *apiclient.Client, string) (*http.Response, error)) func(*cobra.Command, []string) error {
		return func(cmd *cobra.Command, args []string) error {
			client, err := apiclient.NewClient(server, apiclient.WithHTTPClient(httpClient))
			if err != nil {
				return err
			}
			response, err := call(cmd, client, args[0])
			if err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			return printResponse(cmd.OutOrStdout(), response)
		}
	}

	var fail bool
	finish := &cobra.Command{
		Use:   "finish <object-id>",
		Short: "Complete a running task and record an event",
		Args:  cobra.ExactArgs(1),
		RunE: run("finish", func(cmd *cobra.Command, c *apiclient.Client, id string) (*http.Response, error) {
			params := &apiclient.FinishTaskParams{}
			if fail {
				params.Fail = &fail
			}
			return c.FinishTask(cmd.Context(), id, params)
		}),
	}
	finish.Flags().BoolVar(&fail, "fail", false, "Simulate failure before commit to demonstrate rollback")

	root.AddCommand(
		&cobra.Command{
			Use:   "state <object-id>",
			Short: "Read the task status",
			Args:  cobra.ExactArgs(1),
			RunE: run("state", func(cmd *cobra.Command, c *apiclient.Client, id string) (*http.Response, error) {
				return c.GetState(cmd.Context(), id)
			}),
		},
		&cobra.Command{
			Use:   "create <object-id>",
			Short: "Create an object with a pending task (keeps an existing object's status)",
			Args:  cobra.ExactArgs(1),
			RunE: run("create", func(cmd *cobra.Command, c *apiclient.Client, id string) (*http.Response, error) {
				return c.CreateObject(cmd.Context(), id)
			}),
		},
		&cobra.Command{
			Use:   "claim <object-id>",
			Short: "Claim a pending task (claimed=false if already claimed)",
			Args:  cobra.ExactArgs(1),
			RunE: run("claim", func(cmd *cobra.Command, c *apiclient.Client, id string) (*http.Response, error) {
				return c.ClaimTask(cmd.Context(), id)
			}),
		},
		finish,
	)
	return root
}

func printResponse(out io.Writer, response *http.Response) error {
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, body, "", "  "); err != nil {
		return fmt.Errorf("invalid API JSON: %w", err)
	}
	indented.WriteByte('\n')
	_, err = indented.WriteTo(out)
	return err
}
