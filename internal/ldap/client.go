// Package ldap provides a thin wrapper around go-ldap/ldap/v3 that
// performs authentication (bind) and user lookups over LDAPS.
package ldap

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"time"

	goldap "github.com/go-ldap/ldap/v3"
)

// Client is the interface used by handlers to talk to LDAP.
// Defined as an interface so handlers can be tested with mocks.
type Client interface {
	// Authenticate performs an LDAP bind with the given username/password.
	// Returns nil on success, an error on failure.
	Authenticate(ctx context.Context, username, password string) error

	// LookupUser fetches a user by uid and returns its attributes as a map.
	LookupUser(ctx context.Context, uid string) (map[string][]string, error)
}

// Options configures the LDAP client.
type Options struct {
	Host       string
	Port       int
	BaseDN     string
	BindDN     string
	BindPass   string
	UserFilter string // e.g. "(uid=%s)"
	CACertPath string
	Timeout    time.Duration
}

// client is the concrete implementation of Client.
type client struct {
	opts      Options
	tlsConfig *tls.Config
}

// ErrUserNotFound is returned by LookupUser when the uid does not match.
var ErrUserNotFound = errors.New("user not found")

// ErrInvalidCredentials is returned by Authenticate when the bind fails.
var ErrInvalidCredentials = errors.New("invalid credentials")

// New builds a new LDAP client. The CA certificate is loaded once at
// construction so misconfiguration surfaces on startup, not on first request.
func New(opts Options) (Client, error) {
	if opts.Host == "" {
		return nil, errors.New("ldap: host is required")
	}
	if opts.Port == 0 {
		opts.Port = 636
	}
	if opts.UserFilter == "" {
		opts.UserFilter = "(uid=%s)"
	}
	if opts.Timeout == 0 {
		opts.Timeout = 10 * time.Second
	}

	tlsCfg := &tls.Config{
		ServerName: opts.Host,
		MinVersion: tls.VersionTLS12,
	}

	if opts.CACertPath != "" {
		pem, err := os.ReadFile(opts.CACertPath)
		if err != nil {
			return nil, fmt.Errorf("ldap: reading CA cert at %q: %w", opts.CACertPath, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("ldap: no valid PEM certificates found in %q", opts.CACertPath)
		}
		tlsCfg.RootCAs = pool
	}

	return &client{opts: opts, tlsConfig: tlsCfg}, nil
}

// dial opens a new LDAPS connection. A fresh connection is used per request
// to keep the client stateless and safe under concurrency.
func (c *client) dial(ctx context.Context) (*goldap.Conn, error) {
	addr := fmt.Sprintf("%s:%d", c.opts.Host, c.opts.Port)

	dialer := &tls.Dialer{Config: c.tlsConfig}
	ctx, cancel := context.WithTimeout(ctx, c.opts.Timeout)
	defer cancel()
	rawConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("ldap: dialing %s: %w", addr, err)
	}

	conn := goldap.NewConn(rawConn, true)
	conn.Start()
	conn.SetTimeout(c.opts.Timeout)
	return conn, nil
}

// Authenticate binds as the user identified by username using the given
// password. It performs a service-bind first (if configured) to resolve the
// user's DN, then re-binds as the user to verify credentials.
func (c *client) Authenticate(ctx context.Context, username, password string) error {
	if username == "" || password == "" {
		return ErrInvalidCredentials
	}

	conn, err := c.dial(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Service bind (optional) to search for the user's DN.
	if c.opts.BindDN != "" {
		if err := conn.Bind(c.opts.BindDN, c.opts.BindPass); err != nil {
			return fmt.Errorf("ldap: service bind failed: %w", err)
		}
	}

	userDN, err := c.findUserDN(conn, username)
	if err != nil {
		return err
	}

	// Re-bind as the user to verify the password.
	if err := conn.Bind(userDN, password); err != nil {
		if goldap.IsErrorWithCode(err, goldap.LDAPResultInvalidCredentials) {
			return ErrInvalidCredentials
		}
		return fmt.Errorf("ldap: user bind failed: %w", err)
	}
	return nil
}

// LookupUser returns the attributes of a user by uid.
func (c *client) LookupUser(ctx context.Context, uid string) (map[string][]string, error) {
	conn, err := c.dial(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if c.opts.BindDN != "" {
		if err := conn.Bind(c.opts.BindDN, c.opts.BindPass); err != nil {
			return nil, fmt.Errorf("ldap: service bind failed: %w", err)
		}
	}

	entry, err := c.searchUser(conn, uid)
	if err != nil {
		return nil, err
	}

	attrs := make(map[string][]string, len(entry.Attributes))
	for _, a := range entry.Attributes {
		attrs[a.Name] = a.Values
	}
	attrs["dn"] = []string{entry.DN}
	return attrs, nil
}

// findUserDN returns only the DN of a user.
func (c *client) findUserDN(conn *goldap.Conn, username string) (string, error) {
	entry, err := c.searchUser(conn, username)
	if err != nil {
		return "", err
	}
	return entry.DN, nil
}

// searchUser performs the LDAP search with the configured filter.
func (c *client) searchUser(conn *goldap.Conn, username string) (*goldap.Entry, error) {
	filter := fmt.Sprintf(c.opts.UserFilter, goldap.EscapeFilter(username))
	req := goldap.NewSearchRequest(
		c.opts.BaseDN,
		goldap.ScopeWholeSubtree,
		goldap.NeverDerefAliases,
		1, // size limit
		int(c.opts.Timeout.Seconds()),
		false,
		filter,
		[]string{"*"},
		nil,
	)
	result, err := conn.Search(req)
	if err != nil {
		if goldap.IsErrorWithCode(err, goldap.LDAPResultNoSuchObject) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("ldap: search failed: %w", err)
	}
	if len(result.Entries) == 0 {
		return nil, ErrUserNotFound
	}
	return result.Entries[0], nil
}
