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
		Short:         "Create durable objects and run SQL against them",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&server, "server", "http://127.0.0.1:8787", "API base URL")

	run := func(call func(*cobra.Command, *apiclient.Client, []string) (*http.Response, error)) func(*cobra.Command, []string) error {
		return func(cmd *cobra.Command, args []string) error {
			client, err := apiclient.NewClient(server, apiclient.WithHTTPClient(httpClient))
			if err != nil {
				return err
			}
			response, err := call(cmd, client, args)
			if err != nil {
				return fmt.Errorf("%s: %w", cmd.Name(), err)
			}
			return printResponse(cmd.OutOrStdout(), response)
		}
	}

	root.AddCommand(
		&cobra.Command{
			Use:   "create <object-id>",
			Short: "Create an object with an empty database (keeps an existing object's data)",
			Args:  cobra.ExactArgs(1),
			RunE: run(func(cmd *cobra.Command, c *apiclient.Client, args []string) (*http.Response, error) {
				return c.CreateObject(cmd.Context(), args[0])
			}),
		},
		&cobra.Command{
			Use:   "sql <object-id> <sql> [param...]",
			Short: "Run SQL against an object; params fill ? placeholders",
			Long: "Run SQL against an object. Each param fills one ? placeholder. A param that\n" +
				"is valid JSON (42, true, null, \"quoted\") is sent as that value; anything\n" +
				"else is sent as a string.",
			Args: cobra.MinimumNArgs(2),
			RunE: run(func(cmd *cobra.Command, c *apiclient.Client, args []string) (*http.Response, error) {
				params := make([]any, 0, len(args)-2)
				for _, arg := range args[2:] {
					params = append(params, parseParam(arg))
				}
				return c.RunSQL(cmd.Context(), args[0], apiclient.SQLRequest{Sql: args[1], Params: &params})
			}),
		},
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
	if err := json.Indent(&indented, bytes.TrimSpace(body), "", "  "); err != nil {
		return fmt.Errorf("invalid API JSON: %w", err)
	}
	indented.WriteByte('\n')
	_, err = indented.WriteTo(out)
	return err
}

// parseParam reads arg as a JSON value, or as a plain string if it is not one.
// Numbers stay json.Number so integers reach the server exactly.
func parseParam(arg string) any {
	dec := json.NewDecoder(strings.NewReader(arg))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil || dec.More() {
		return arg
	}
	return v
}
