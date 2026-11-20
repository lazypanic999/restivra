package source

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfilesKeepSecretsOutOfDescriptorsAndErrors(t *testing.T) {
	p := Profile{ID: "offsite", Name: "Offsite backups", Region: "eu-central-1", Endpoint: "https://s3.example.com", AccessKeyID: "private-access-sentinel", SecretAccessKey: "private-secret-sentinel", SessionToken: "private-session-sentinel"}
	filename := filepath.Join(t.TempDir(), "sources.json")
	write := func(value any) {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if os.WriteFile(filename, raw, 0600) != nil {
			t.Fatal("write fixture")
		}
	}
	write(Profiles{S3: []Profile{p}})
	loaded, err := LoadProfiles(filename)
	if err != nil {
		t.Fatal(err)
	}
	public, _ := json.Marshal(loaded.Public())
	for _, secret := range []string{p.AccessKeyID, p.SecretAccessKey, p.SessionToken, p.Endpoint} {
		if strings.Contains(string(public), secret) {
			t.Fatal("public descriptors contain private settings")
		}
	}
	if string(public) != `[{"id":"offsite","name":"Offsite backups"}]` {
		t.Fatal("unexpected descriptor")
	}
	for _, mutate := range []func(*Profile){
		func(p *Profile) { p.ID = "../unsafe" }, func(p *Profile) { p.Name = "" }, func(p *Profile) { p.Endpoint = "https://user:private-secret-sentinel@example.com" }, func(p *Profile) { p.Endpoint = "http://example.com" }, func(p *Profile) { p.Endpoint = "https://example.com?token=private-secret-sentinel" }, func(p *Profile) { p.Region = "invalid/region" }, func(p *Profile) { p.AccessKeyID = "" },
	} {
		bad := p
		mutate(&bad)
		write(Profiles{S3: []Profile{bad}})
		_, err = LoadProfiles(filename)
		if err == nil {
			t.Fatal("invalid private profile accepted")
		}
		if strings.Contains(err.Error(), "sentinel") {
			t.Fatal("decoder/validation error leaked credential")
		}
	}
	write(Profiles{S3: []Profile{p, p}})
	if _, err = LoadProfiles(filename); err == nil {
		t.Fatal("duplicate profile accepted")
	}
	if err = os.WriteFile(filename, []byte(`{"s3":[],"secret-secret-sentinel":"x"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadProfiles(filename); err == nil || strings.Contains(err.Error(), "sentinel") {
		t.Fatal("unknown fields accepted or exposed")
	}
	if err = os.WriteFile(filename, []byte(`{"s3":[]} {"s3":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadProfiles(filename); err == nil {
		t.Fatal("trailing object accepted")
	}
	p.AllowHTTP = true
	p.Endpoint = "http://127.0.0.1:9000"
	write(Profiles{S3: []Profile{p}})
	if _, err = LoadProfiles(filename); err != nil {
		t.Fatal("explicit trusted HTTP profile rejected")
	}
}

func TestSelectionsCannotContainCredentials(t *testing.T) {
	for _, selection := range []Selection{{Kind: "unexpected"}, {Kind: "local", Profile: "offsite"}, {Kind: "s3", Profile: "../key", Bucket: "backups"}, {Kind: "s3", Profile: "offsite", Bucket: "127.0.0.1"}, {Kind: "s3", Profile: "offsite", Bucket: "bad..bucket"}} {
		if selection.Validate("daily/*.dump") == nil {
			t.Fatal("invalid selection accepted")
		}
	}
	local := Selection{}
	if local.Validate("daily/*.dump") != nil || local.Kind != "local" {
		t.Fatal("legacy local normalization failed")
	}
	remote := Selection{Kind: "s3", Profile: "offsite", Bucket: "customer-backups"}
	if remote.Validate("daily/*.dump") != nil {
		t.Fatal("valid remote selection rejected")
	}
	if remote.Validate("daily/[bad") == nil {
		t.Fatal("malformed pattern accepted")
	}
}
