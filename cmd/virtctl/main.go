package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const defaultServerURL = "http://127.0.0.1:8080"

var version = "dev"

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr, http.DefaultClient); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer, client *http.Client) error {
	if len(args) == 0 {
		printUsage(stderr)
		return errors.New("command is required")
	}

	switch args[0] {
	case "version":
		fmt.Fprintf(stdout, "virtctl %s\n", version)
		return nil
	case "health":
		return runHealth(ctx, args[1:], stdout, client)
	case "host-capabilities":
		return runHostCapabilities(ctx, args[1:], stdout, client)
	case "vm-list":
		return runVMList(ctx, args[1:], stdout, client)
	case "preflight":
		return runPreflight(ctx, args[1:], stdout, client)
	case "help", "-h", "--help":
		printUsage(stdout)
		return nil
	default:
		printUsage(stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runHealth(ctx context.Context, args []string, stdout io.Writer, client *http.Client) error {
	flags := flag.NewFlagSet("health", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	server := flags.String("server", serverFromEnv(), "warm-migrationd server URL")
	if err := flags.Parse(args); err != nil {
		return err
	}

	body, status, err := requestJSON(ctx, client, http.MethodGet, *server, "/healthz", nil)
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, string(body))
	if status < 200 || status > 299 {
		return fmt.Errorf("health request failed with HTTP %d", status)
	}
	return nil
}

func runHostCapabilities(ctx context.Context, args []string, stdout io.Writer, client *http.Client) error {
	flags := flag.NewFlagSet("host-capabilities", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	server := flags.String("server", serverFromEnv(), "warm-migrationd server URL")
	if err := flags.Parse(args); err != nil {
		return err
	}
	return requestAndPrint(ctx, client, http.MethodGet, *server, "/host/capabilities", nil, stdout, "host capabilities")
}

func runVMList(ctx context.Context, args []string, stdout io.Writer, client *http.Client) error {
	flags := flag.NewFlagSet("vm-list", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	server := flags.String("server", serverFromEnv(), "warm-migrationd server URL")
	if err := flags.Parse(args); err != nil {
		return err
	}
	return requestAndPrint(ctx, client, http.MethodGet, *server, "/host/vms", nil, stdout, "VM list")
}

func runPreflight(ctx context.Context, args []string, stdout io.Writer, client *http.Client) error {
	flags := flag.NewFlagSet("preflight", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	server := flags.String("server", serverFromEnv(), "warm-migrationd server URL")
	vmID := flags.String("vm-id", "", "VM identifier")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*vmID) == "" {
		return fmt.Errorf("--vm-id is required")
	}

	payload := fmt.Sprintf(`{"vm_id":%q}`, *vmID)
	return requestAndPrint(ctx, client, http.MethodPost, *server, "/migrations/preflight", []byte(payload), stdout, "preflight")
}

func requestAndPrint(ctx context.Context, client *http.Client, method string, server string, path string, payload []byte, stdout io.Writer, operation string) error {
	body, status, err := requestJSON(ctx, client, method, server, path, payload)
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, string(body))
	if status < 200 || status > 299 {
		return fmt.Errorf("%s request failed with HTTP %d", operation, status)
	}
	return nil
}

func requestJSON(ctx context.Context, client *http.Client, method string, rawServer string, path string, payload []byte) ([]byte, int, error) {
	endpoint, err := joinURL(rawServer, path)
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, 0, err
	}
	return body, res.StatusCode, nil
}

func joinURL(rawServer string, path string) (string, error) {
	rawServer = strings.TrimRight(strings.TrimSpace(rawServer), "/")
	if rawServer == "" {
		return "", fmt.Errorf("server URL is required")
	}
	parsed, err := url.Parse(rawServer)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("server URL must include scheme and host")
	}
	return rawServer + path, nil
}

func serverFromEnv() string {
	if value := strings.TrimSpace(os.Getenv("VIRTCTL_SERVER")); value != "" {
		return value
	}
	return defaultServerURL
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: virtctl <version|health|host-capabilities|vm-list|preflight>")
	fmt.Fprintln(w, "  virtctl health [--server http://127.0.0.1:8080]")
	fmt.Fprintln(w, "  virtctl host-capabilities [--server http://127.0.0.1:8080]")
	fmt.Fprintln(w, "  virtctl vm-list [--server http://127.0.0.1:8080]")
	fmt.Fprintln(w, "  virtctl preflight --vm-id <vm-id> [--server http://127.0.0.1:8080]")
}
