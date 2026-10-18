package onboarding

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"fingo/internal/adapter/mariadb/conn"
)

// DefaultHost is where a database on this machine is looked for. The screen
// calls detection without an address, so these are what it gets.
const DefaultHost = "127.0.0.1"

// detectTimeout bounds a detection.
//
// Short on purpose. This runs while an operator is looking at a form, and a
// machine whose firewall drops packets rather than refusing them would otherwise
// hold the screen for the driver's default. Three seconds is long enough for a
// loopback or LAN server to answer and short enough to read as "not there".
const detectTimeout = 3 * time.Second

// handshakeProtocol is the protocol version a MySQL-family server greets with.
//
// Every version of MySQL and MariaDB still speaks protocol 10 — the number has
// not moved in twenty years — so it is a reliable "this is one of ours" check
// and not a version test.
const handshakeProtocol = 10

// compatibilityPrefix is what MariaDB puts in front of its version string.
//
// MariaDB reports itself as "5.5.5-10.11.14-MariaDB": the leading 5.5.5 is a
// fiction kept for clients old enough to refuse a 10.x server. Showing it to an
// operator would tell them they have MariaDB 5.5, which they do not.
const compatibilityPrefix = "5.5.5-"

// Local is what was found on this machine.
//
// It is a report and not an error. "There is no database here" is an answer to
// the question the screen asked, and the commonest one on a machine that has
// never had a server installed — so it is a value with a reason in it, not a
// failure the caller has to unwrap.
type Local struct {
	// Found is whether a MySQL-family server answered.
	Found bool `json:"found"`

	// Host and Port are the address that was looked at, so the screen can say
	// which one it means.
	Host string `json:"host"`
	Port string `json:"port"`

	// Version is the server's own version string, with MariaDB's compatibility
	// prefix removed. Empty when nothing was found.
	Version string `json:"version,omitempty"`

	// Problem says why nothing was found, in words an operator can act on.
	Problem string `json:"problem,omitempty"`
}

// Detect looks for a database server on this machine.
//
// It connects and reads the greeting the server sends before any credentials are
// offered. That is the whole reason it can work at all: the server announces
// itself, and its version, before it asks who you are. A detection built on
// opening a database would need the username and password the screen is still
// asking for, and would report "nothing there" for a server whose login it did
// not have — which is precisely the wrong answer.
//
// It answers for a server, not for a database. Whether the books exist on that
// server is Probe's question, and it is asked once there are credentials to ask
// it with.
func Detect(ctx context.Context, host, port string) Local {
	out := Local{Host: strings.TrimSpace(host), Port: strings.TrimSpace(port)}

	if out.Host == "" {
		out.Host = DefaultHost
	}
	if out.Port == "" {
		out.Port = conn.DefaultPort
	}

	ctx, cancel := context.WithTimeout(ctx, detectTimeout)
	defer cancel()

	version, err := greet(ctx, out.Host, out.Port)
	if err != nil {
		out.Problem = problemAt(out.Host, out.Port, err)

		return out
	}

	out.Found = true
	out.Version = strings.TrimPrefix(version, compatibilityPrefix)

	return out
}

// greet reads the first packet a server sends.
//
// The MySQL wire format opens with a 3-byte little-endian payload length and a
// sequence byte, and the payload is the handshake: a protocol byte, then the
// server version as a NUL-terminated string. Nothing before that point requires
// authentication.
func greet(ctx context.Context, host, port string) (string, error) {
	dialer := net.Dialer{Timeout: detectTimeout}

	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, port))
	if err != nil {
		return "", err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(detectTimeout)); err != nil {
		return "", err
	}

	var header [4]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return "", fmt.Errorf("the connection was accepted and then went quiet: %w", err)
	}

	length := int(header[0]) | int(header[1])<<8 | int(header[2])<<16
	if length < 2 {
		return "", fmt.Errorf("the greeting is %d bytes long, which is not a handshake", length)
	}

	payload := make([]byte, length)
	if n, err := io.ReadFull(conn, payload); err != nil {
		return "", fmt.Errorf("the greeting stopped after %d of %d bytes: %w", n, length, err)
	}

	if payload[0] != handshakeProtocol {
		return "", fmt.Errorf("whatever is listening speaks protocol %d, not MySQL's %d",
			payload[0], handshakeProtocol)
	}

	rest := payload[1:]

	end := bytes.IndexByte(rest, 0)
	if end < 0 {
		return "", errors.New("the greeting named no version")
	}

	return string(rest[:end]), nil
}

// problemAt says what went wrong in terms of the address, because the address is
// the thing the operator typed.
func problemAt(host, port string, err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Sprintf("nothing answered at %s:%s within %s", host, port, detectTimeout)
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return fmt.Sprintf("nothing is listening at %s:%s (%v)", host, port, opErr.Err)
	}

	return fmt.Sprintf("%s:%s could not be used: %v", host, port, err)
}
