package auth

import (
	"strings"
	"testing"
)

func TestClientCredentials(t *testing.T) {
	cid := GetClientID()
	if !strings.HasSuffix(cid, ".apps.googleusercontent.com") {
		t.Fatalf("unexpected client id: %s", cid)
	}

	csec := GetClientSecret()
	if !strings.HasPrefix(csec, "GOCSPX-") {
		t.Fatalf("unexpected client secret: %s", csec)
	}
}
