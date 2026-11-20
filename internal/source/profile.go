package source

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var profileID = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
var bucketName = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)
var regionName = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)

// Selection is safe to persist in immutable job settings. Secrets live in the
// worker's private profile file; the web API accepts no credential fields.
type Selection struct {
	Kind    string `json:"kind"`
	Profile string `json:"profile,omitempty"`
	Bucket  string `json:"bucket,omitempty"`
}

func (s *Selection) Validate(pattern string) error {
	if s.Kind == "" {
		s.Kind = "local"
	}
	switch s.Kind {
	case "local":
		if s.Profile != "" || s.Bucket != "" {
			return errors.New("local sources cannot specify an S3 profile or bucket")
		}
	case "s3":
		if !profileID.MatchString(s.Profile) || !bucketName.MatchString(s.Bucket) || strings.Contains(s.Bucket, "..") || net.ParseIP(s.Bucket) != nil {
			return errors.New("invalid S3 source profile or bucket")
		}
		if len(pattern) == 0 || len(pattern) > 1024 || strings.HasPrefix(pattern, "/") || strings.ContainsAny(pattern, "\\\x00\r\n") {
			return errors.New("invalid S3 object pattern")
		}
		if _, err := path.Match(pattern, ""); err != nil {
			return errors.New("invalid S3 object pattern")
		}
	default:
		return errors.New("unsupported backup source")
	}
	return nil
}

type PublicProfile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type Profile struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Endpoint        string `json:"endpoint"`
	Region          string `json:"region"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	SessionToken    string `json:"session_token,omitempty"`
	PathStyle       bool   `json:"path_style"`
	AllowHTTP       bool   `json:"allow_http,omitempty"`
}
type Profiles struct {
	S3 []Profile `json:"s3"`
}

func LoadProfiles(filename string) (Profiles, error) {
	result := Profiles{S3: []Profile{}}
	if filename == "" {
		return result, nil
	}
	f, err := os.Open(filename)
	// Optional on legacy local-only installations. Any present but invalid file
	// fails startup; no secret-bearing decoder error is logged.
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return result, errors.New("could not read private source profiles")
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil || !stat.Mode().IsRegular() || stat.Size() > 1<<20 {
		return result, errors.New("invalid private source profile file")
	}
	d := json.NewDecoder(io.LimitReader(f, 1<<20))
	d.DisallowUnknownFields()
	if d.Decode(&result) != nil || d.Decode(new(any)) != io.EOF || len(result.S3) > 20 {
		return Profiles{}, errors.New("invalid private source profiles")
	}
	seen := map[string]bool{}
	for i := range result.S3 {
		p := &result.S3[i]
		p.Name = strings.TrimSpace(p.Name)
		if !profileID.MatchString(p.ID) || seen[p.ID] || len(p.Name) == 0 || len(p.Name) > 120 || strings.ContainsAny(p.Name, "\x00\r\n") || !regionName.MatchString(p.Region) || len(p.AccessKeyID) == 0 || len(p.AccessKeyID) > 256 || len(p.SecretAccessKey) == 0 || len(p.SecretAccessKey) > 1024 || len(p.SessionToken) > 8192 {
			return Profiles{}, errors.New("invalid private source profile settings")
		}
		seen[p.ID] = true
		if p.Endpoint != "" {
			u, e := url.Parse(p.Endpoint)
			if e != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || (u.Scheme != "https" && !(u.Scheme == "http" && p.AllowHTTP)) {
				return Profiles{}, errors.New("source endpoints require HTTPS; explicitly allow HTTP only on trusted private networks")
			}
		}
	}
	return result, nil
}

func (p Profiles) Public() []PublicProfile {
	result := []PublicProfile{}
	for _, v := range p.S3 {
		result = append(result, PublicProfile{ID: v.ID, Name: v.Name})
	}
	return result
}

func (p Profiles) Find(id string) (Profile, bool) {
	for _, v := range p.S3 {
		if v.ID == id {
			return v, true
		}
	}
	return Profile{}, false
}

func (p Profile) Client() *s3.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.ResponseHeaderTimeout = 20 * time.Second
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	options := s3.Options{Region: p.Region, Credentials: credentials.NewStaticCredentialsProvider(p.AccessKeyID, p.SecretAccessKey, p.SessionToken), UsePathStyle: p.PathStyle, HTTPClient: client, RetryMaxAttempts: 2}
	if p.Endpoint != "" {
		options.BaseEndpoint = aws.String(p.Endpoint)
	}
	return s3.New(options)
}
