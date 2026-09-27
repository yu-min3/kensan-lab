// private-route-probe checks only direct TCP reachability from a separate
// host. It cannot certify tunnel, proxy, firewall, or CI publication routes.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
	"time"
)

type result struct {
	Target    string `json:"target"`
	Status    string `json:"status"`
	ErrorKind string `json:"error_kind,omitempty"`
}

type report struct {
	SchemaVersion int       `json:"schema_version"`
	CheckedAt     time.Time `json:"checked_at"`
	SourceHost    string    `json:"source_host"`
	Scope         string    `json:"scope"`
	Results       []result  `json:"results"`
}

func classify(ctx context.Context, address string) result {
	r := result{Target: address}
	conn, err := (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, "tcp", address)
	if err == nil {
		_ = conn.Close()
		r.Status = "reachable"
		return r
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		r.Status = "direct_refused"
		return r
	}
	r.Status = "unknown"
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, syscall.ETIMEDOUT):
		r.ErrorKind = "timeout"
	case errors.Is(err, syscall.ENETUNREACH), errors.Is(err, syscall.EHOSTUNREACH):
		r.ErrorKind = "no_route"
	default:
		r.ErrorKind = "dial_error"
	}
	return r
}

func validIP(value string, family int) bool {
	ip := net.ParseIP(value)
	if ip == nil || ip.IsLoopback() || ip.IsUnspecified() {
		return false
	}
	if family == 4 {
		return ip.To4() != nil && ip.IsPrivate()
	}
	return ip.To4() == nil && ip.IsGlobalUnicast() && !ip.IsPrivate()
}

func run(args []string) int {
	flags := flag.NewFlagSet("private-route-probe", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	lan := flags.String("lan-ip", "", "sense private LAN IPv4 from the reviewed inventory")
	global6 := flags.String("global-ipv6", "", "sense global IPv6 from the reviewed inventory")
	port := flags.Int("port", 8787, "reviewed private listener port")
	if flags.Parse(args) != nil || flags.NArg() != 0 || !validIP(*lan, 4) || !validIP(*global6, 6) || *port != 8787 {
		fmt.Fprintln(os.Stderr, "provide reviewed private LAN IPv4 and global IPv6; port must be 8787")
		return 2
	}
	host, err := os.Hostname()
	if err != nil || strings.EqualFold(strings.Split(host, ".")[0], "sense") {
		fmt.Fprintln(os.Stderr, "run from a separate host, not sense")
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	r := report{SchemaVersion: 1, CheckedAt: time.Now().UTC(), SourceHost: host, Scope: "direct_tcp_only; tunnels/proxies/CI/Internet routes unverified", Results: []result{
		classify(ctx, net.JoinHostPort(*lan, "8787")),
		classify(ctx, net.JoinHostPort(*global6, "8787")),
	}}
	if err := json.NewEncoder(os.Stdout).Encode(r); err != nil {
		return 1
	}
	for _, item := range r.Results {
		if item.Status != "direct_refused" {
			return 1
		}
	}
	return 0
}

func main() { os.Exit(run(os.Args[1:])) }
