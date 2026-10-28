package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/nickstrad/mini-durable-objects/internal/apiclient"
	"github.com/nickstrad/mini-durable-objects/internal/errs"
	"github.com/spf13/cobra"
)

func main() {
	cmd := newRootCommand(&http.Client{Timeout: 10 * time.Second})
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, userMessage(err))
		os.Exit(1)
	}
}

// userMessage is what main prints. An Internal error also shows its cause,
// because the operator running the CLI needs it (for example a network error).
func userMessage(err error) string {
	e, ok := errors.AsType[*errs.Error](err)
	if !ok {
		return err.Error()
	}
	if e.Kind == errs.Internal && e.Inner != nil {
		return e.Message + ": " + e.Inner.Error()
	}
	return e.Message
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
			op := "mdo " + cmd.Name()
			client, err := apiclient.NewClient(server, apiclient.WithHTTPClient(httpClient))
			if err != nil {
				return errs.Wrap(err, errs.Internal, op, "invalid server URL")
			}
			response, err := call(cmd, client, args)
			if err != nil {
				return errs.Wrap(err, errs.Internal, op, "cannot reach the server")
			}
			return printResponse(cmd.OutOrStdout(), op, response)
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

func printResponse(out io.Writer, op string, response *http.Response) error {
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return errs.Wrap(err, errs.Internal, op, "cannot read the response")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		msg := fmt.Sprintf("HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
		return errs.Wrap(nil, kindFromStatus(response.StatusCode), op, msg)
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, bytes.TrimSpace(body), "", "  "); err != nil {
		return errs.Wrap(err, errs.Internal, op, "invalid API JSON")
	}
	indented.WriteByte('\n')
	_, err = indented.WriteTo(out)
	if err != nil {
		return errs.Wrap(err, errs.Internal, op, "cannot write the output")
	}
	return nil
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

// kindFromStatus maps an HTTP status from the server back to an error Kind.
// The mapping loses detail: the server sends both Invalid and BadQuery as 400.
func kindFromStatus(code int) errs.Kind {
	switch code {
	case http.StatusBadRequest:
		return errs.Invalid
	case http.StatusNotFound:
		return errs.NotFound
	case http.StatusServiceUnavailable:
		return errs.Canceled
	}
	return errs.Internal
}
