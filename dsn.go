package main

import (
	"strings"
	"fmt"
	urllib "net/url"
	"errors"
)

type DSN struct {
	DSN string
	Scheme string
	Host string
	Port string
	Hostname string
	PublicKey string
	SecretKey string
	ProjectID string
	AuthHeader string
	StoreAPI string
}

func (dsn *DSN) MarshalJSON() ([]byte, error) {
	return []byte("\"" + dsn.DSN + "\""), nil
}

func (dsn *DSN) UnmarshalJSON(data []byte) error {
	dsn.DSN = string(data[1:len(data)-1])
	return dsn.Parse()
}

func (dsn *DSN) Parse() error {
	if dsn.DSN == "" {
		return errors.New("DSN must not be empty")
	}

	// Parse URL
	url, err := urllib.Parse(dsn.DSN)
	if err != nil {
		return err
	}

	// Set scheme
	dsn.Scheme = url.Scheme

	// Set host and port
	dsn.Host = url.Host
	dsn.Port = url.Port()

	// Set hostname (host:port)
	dsn.Hostname = url.Hostname()

	// Set public key
	if url.User == nil {
		return errors.New("DSN missing public key and/or password")
	}
	dsn.PublicKey = url.User.Username()

	// Set secret key
	var ok bool
	dsn.SecretKey, ok = url.User.Password()
	if !ok {
		return errors.New("DSN missing private key")
	}

	// Set project ID
	if idx := strings.LastIndex(url.Path, "/"); idx != -1 {
		dsn.ProjectID = url.Path[idx+1:]
	}
	if dsn.ProjectID == "" {
		return errors.New("DSN missing project id")
	}

	// Set auth header
	dsn.AuthHeader = fmt.Sprintf("Sentry sentry_version=6, sentry_key=%s, sentry_secret=%s", dsn.PublicKey, dsn.SecretKey)

	// Set store API endpoint URL
	dsn.StoreAPI = fmt.Sprintf("%s://%s:%s@%s/api/%s/store/", dsn.Scheme, dsn.PublicKey, dsn.SecretKey, dsn.Host, dsn.ProjectID)

	return nil
}
