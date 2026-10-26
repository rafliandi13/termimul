package sshx

import (
	"testing"

	"golang.org/x/crypto/ssh"

	"termimus/internal/sshtest"
	"termimus/internal/vault"
)

func TestLegacyKeyPathAuth(t *testing.T) {
	cases := []struct {
		name, keyPass, givenPass string
		wantKeyErr               bool
	}{
		{"plain key", "", "", false},
		{"encrypted key with passphrase", "rahasia", "rahasia", false},
		{"encrypted key without passphrase", "rahasia", "", true},
		{"encrypted key wrong passphrase", "rahasia", "salah", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path, pub := sshtest.WriteKey(t, tc.keyPass)
			addr := sshtest.StartServer(t, pub)
			auth, err := AuthMethods(vault.Host{AuthType: "key", KeyPath: path}, vault.Secrets{Passphrase: tc.givenPass})
			if tc.wantKeyErr {
				if err == nil {
					t.Fatal("expected key loading to fail")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := sshtest.Dial(addr, auth); err != nil {
				t.Fatalf("login failed: %v", err)
			}
		})
	}
}

func TestServerRejectsOtherKey(t *testing.T) {
	path, _ := sshtest.WriteKey(t, "")
	_, otherPub := sshtest.WriteKey(t, "")
	addr := sshtest.StartServer(t, otherPub)
	auth, err := AuthMethods(vault.Host{AuthType: "key", KeyPath: path}, vault.Secrets{})
	if err != nil {
		t.Fatal(err)
	}
	if err := sshtest.Dial(addr, auth); err == nil {
		t.Fatal("server accepted a key it doesn't know")
	}
}

func TestKeychainKeysCanLogIn(t *testing.T) {
	for _, kind := range []string{"ed25519", "rsa", "ecdsa"} {
		t.Run(kind, func(t *testing.T) {
			store, err := vault.Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			priv, err := vault.GeneratePrivateKey(kind)
			if err != nil {
				t.Fatal(err)
			}
			k, err := store.AddKey("laptop", priv)
			if err != nil {
				t.Fatal(err)
			}
			pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(k.PublicKey))
			if err != nil {
				t.Fatalf("public key line %q: %v", k.PublicKey, err)
			}
			addr := sshtest.StartServer(t, pub)

			h, err := store.SaveHost(vault.HostInput{Host: vault.Host{Address: "x", Username: "u", AuthType: "key", KeyID: k.ID}})
			if err != nil {
				t.Fatal(err)
			}
			host, sec, err := store.Host(h.ID)
			if err != nil {
				t.Fatal(err)
			}
			auth, err := AuthMethods(host, sec)
			if err != nil {
				t.Fatal(err)
			}
			if err := sshtest.Dial(addr, auth); err != nil {
				t.Fatalf("login failed: %v", err)
			}
		})
	}
}

func TestPasswordRequired(t *testing.T) {
	if _, err := AuthMethods(vault.Host{AuthType: "password"}, vault.Secrets{}); err == nil {
		t.Fatal("expected an error when no password is stored")
	}
}
