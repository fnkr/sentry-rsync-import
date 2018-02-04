package main

import (
	"testing"
	"fmt"
)

const (
	TestValidPublicKey = "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
	TestValidSecretKey = "yyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy"
	TestValidScheme    = "https"
	TestValidHostname  = "hostname"
	TestValidPort      = "1234"
	TestValidProjectId = "5678"

)

func TestDSN(t *testing.T) {
	dsnstr := TestValidScheme + "://" + TestValidPublicKey + ":" + TestValidSecretKey + "@" + TestValidHostname + ":" + TestValidPort + "/" + TestValidProjectId
	dsn := DSN{DSN: dsnstr}
	err := dsn.Parse()
	if err != nil {
		t.Error(err.Error())
	}
	if dsn.Scheme != TestValidScheme {
		t.Error("Scheme does not match")
	}
	if dsn.Hostname != TestValidHostname {
		t.Error("Hostname does not match")
	}
	if dsn.Host != TestValidHostname + ":" + TestValidPort {
		t.Error("Host does not match")
	}
	if dsn.Port != TestValidPort {
		t.Error("Port does not match")
	}
	if dsn.PublicKey != TestValidPublicKey {
		t.Error("PublicKey does not match")
	}
	if dsn.SecretKey != TestValidSecretKey {
		t.Error("SecretKey does not match")
	}
	if dsn.ProjectID != TestValidProjectId {
		t.Error("ProjectID does not match")
	}
	if dsn.DSN != dsnstr {
		t.Error("DSN does not match")
	}
	if dsn.AuthHeader != fmt.Sprintf("Sentry sentry_version=6, sentry_key=%s, sentry_secret=%s", TestValidPublicKey, TestValidSecretKey) {
		t.Error("AuthHeader does not match")
	}
	if dsn.StoreAPI != (dsn.Scheme + "://" + dsn.PublicKey + ":" + dsn.SecretKey + "@" + dsn.Hostname + ":" + dsn.Port + "/api/" + dsn.ProjectID + "/store/") {
		t.Error(dsn.StoreAPI)
	}
}
