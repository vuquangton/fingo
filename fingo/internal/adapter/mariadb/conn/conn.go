// Package conn describes how to reach the MariaDB server that holds client
// databases.
//
// FinGo keeps one database per client (see docs/requirements-v1.md §1.1), so
// the database name is deliberately NOT part of Config. It is supplied per
// connection through DSN. That keeps one Config usable whether a practice runs
// a single server holding N client schemas, or N separate instances.
package conn

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/go-sql-driver/mysql"
)

// Config describes how to reach the MariaDB server.
//
// The JSON tags are lowercase because this struct is what an installation's
// saved connection is made of, and that file is meant to be readable — and
// editable — by the person who owns the machine. Go's default field names would
// put "Password" in a file without saying what it is a password to.
type Config struct {
	Host     string            `json:"host"`
	Port     string            `json:"port"`
	User     string            `json:"user"`
	Password string            `json:"password"`
	Params   map[string]string `json:"params,omitempty"`
}

// DSN builds a MySQL DSN for one client's database.
//
// The string is assembled by the driver rather than by concatenation, so
// passwords containing '@', ':' or '/' are escaped correctly.
func (c Config) DSN(database string) string {
	cfg := mysql.NewConfig()
	cfg.User = c.User
	cfg.Passwd = c.Password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(c.Host, c.Port)
	cfg.DBName = database
	cfg.Params = c.Params

	return cfg.FormatDSN()
}

// DefaultPort is assumed when the environment does not name one. Without it a
// host would resolve to an unusable address such as "127.0.0.1:".
const DefaultPort = "3306"

// FromEnv builds a Config from environment variables.
//
// lookup is injected rather than reading os.Getenv directly, so callers can
// supply a .env file and tests need not touch the process environment.
func FromEnv(lookup func(string) string) (Config, error) {
	host := strings.TrimSpace(lookup("DB_HOST"))
	port := strings.TrimSpace(lookup("DB_PORT"))
	user := strings.TrimSpace(lookup("DB_USER"))
	password := lookup("DB_PASSWORD") // never trimmed: spaces may be significant

	if host == "" {
		return Config{}, errors.New("DB_HOST is required")
	}
	if user == "" {
		return Config{}, errors.New("DB_USER is required")
	}
	if port == "" {
		port = DefaultPort
	}

	params, err := ParseParams(lookup("DB_PARAMS"))
	if err != nil {
		return Config{}, err
	}

	return Config{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		Params:   params,
	}, nil
}

// ParseParams turns "charset=utf8mb4&parseTime=True" into a map.
//
// Exported because the connection form collects the parameters as one field and
// has to end up with the same map the environment produces. A second parser
// behind the screen would be a second set of rules about what a parameter is,
// and the two would disagree about the case that matters: parseTime.
//
// A parameter with no value is an error rather than a silent omission: losing
// parseTime would quietly change how every date column is read.
func ParseParams(raw string) (map[string]string, error) {
	params := make(map[string]string)

	for _, pair := range strings.Split(strings.TrimSpace(raw), "&") {
		if pair == "" {
			continue
		}

		key, value, found := strings.Cut(pair, "=")
		if !found || key == "" {
			return nil, fmt.Errorf("malformed DSN parameter %q: expected key=value", pair)
		}

		params[key] = value
	}

	return params, nil
}

// ParseDotEnv reads KEY=VALUE lines.
//
// Blank lines and lines starting with '#' are skipped. Surrounding single or
// double quotes are stripped. Separators inside a value are kept, because
// DB_PARAMS legitimately contains '&'.
//
// Values are trimmed of surrounding whitespace and a line with no '=' is an
// error rather than a silent omission.
func ParseDotEnv(r io.Reader) (map[string]string, error) {
	env := make(map[string]string)
	scanner := bufio.NewScanner(r)

	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			return nil, fmt.Errorf(".env line %d is not a KEY=VALUE pair: %q", lineNo, line)
		}

		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf(".env line %d has an empty key", lineNo)
		}

		env[key] = stripQuotes(strings.TrimSpace(value))
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return env, nil
}

func stripQuotes(s string) string {
	if len(s) >= 2 {
		first, last := s[0], s[len(s)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// FromEnvFile reads a .env file into a Config and the client database that this
// installation opens by default.
//
// FinGo keeps one database per client (docs/requirements-v1.md §1.1), so the
// database name is not part of Config. Until a practice-level client registry
// exists, DB_NAME names the client that opens at startup.
//
// A missing or malformed file is an error. Falling back to an empty environment
// would surface much later as an unexplained connection failure.
func FromEnvFile(path string) (Config, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, "", fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	env, err := ParseDotEnv(f)
	if err != nil {
		return Config{}, "", fmt.Errorf("parse %s: %w", path, err)
	}

	lookup := func(k string) string { return env[k] }

	cfg, err := FromEnv(lookup)
	if err != nil {
		return Config{}, "", fmt.Errorf("%s: %w", path, err)
	}

	database := strings.TrimSpace(lookup("DB_NAME"))
	if database == "" {
		return Config{}, "", fmt.Errorf("%s: DB_NAME is required — a client's books live in their own database", path)
	}

	return cfg, database, nil
}
