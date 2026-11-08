// Package requests persists CLI task submissions independently of their source files.
package requests

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"hvx-cli/internal/config"
)

type Record struct {
	ID               string          `json:"requestId"`
	Key              string          `json:"idempotencyKey"`
	Server           string          `json:"server"`
	Mode             string          `json:"mode"`
	CredentialDigest string          `json:"credentialDigest"`
	Insecure         bool            `json:"insecure"`
	Body             []byte          `json:"body"`
	Response         json.RawMessage `json:"response,omitempty"`
	TaskID           string          `json:"taskId,omitempty"`
	PayloadDigest    string          `json:"payloadDigest"`
	State            string          `json:"state"`
	CreatedAt        time.Time       `json:"createdAt"`
}

func CredentialDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func directory() (string, error) {
	path, err := config.Path()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), "requests"), nil
}
func New(server, mode, token string, insecure bool, body []byte, key string) (*Record, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return nil, err
	}
	id := hex.EncodeToString(random)
	if key == "" {
		key = id
	}
	r := &Record{ID: id, Key: key, Server: strings.TrimRight(server, "/"), Mode: mode, CredentialDigest: CredentialDigest(token), Insecure: insecure, Body: append([]byte(nil), body...), State: "prepared", CreatedAt: time.Now().UTC()}
	r.PayloadDigest = CredentialDigest(string(body))
	if err := Save(r); err != nil {
		return nil, err
	}
	return r, nil
}

var identifier = regexp.MustCompile(`^[a-f0-9]{32}$`)

func pathFor(id string) (string, error) {
	if !identifier.MatchString(id) {
		return "", fmt.Errorf("invalid request ID")
	}
	dir, err := directory()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(dir, id+".json"), nil
}
func Lock(id string) (func(), error) {
	path, err := pathFor(id)
	if err != nil {
		return nil, err
	}
	guard := flock.New(path + ".lock")
	ok, err := guard.TryLock()
	if err != nil {
		return nil, err
	}
	if !ok {
		guard.Close()
		return nil, fmt.Errorf("request already being submitted by another process")
	}
	return func() { _ = guard.Close() }, nil
}
func Save(r *Record) error {
	path, err := pathFor(r.ID)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".request-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err = json.NewEncoder(tmp).Encode(r); err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		dir, err := os.Open(filepath.Dir(path))
		if err != nil {
			return err
		}
		defer dir.Close()
		return dir.Sync()
	}
	return nil
}
func Load(id string) (*Record, error) {
	path, err := pathFor(id)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Record
	if err = json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	if r.ID != id || r.Key == "" || !json.Valid(r.Body) || r.PayloadDigest != CredentialDigest(string(r.Body)) {
		return nil, fmt.Errorf("invalid request record; refusing to replace uncertain state")
	}
	return &r, nil
}
func List() ([]Record, error) {
	dir, err := directory()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []Record{}, nil
	}
	if err != nil {
		return nil, err
	}
	result := []Record{}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		r, err := Load(strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		result = append(result, *r)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })
	return result, nil
}
