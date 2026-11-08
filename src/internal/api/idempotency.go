package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/gofrs/flock"
)

// pendingTaskKey stores only a random key under a digest of endpoint, credential,
// and exact request bytes. Neither credentials nor task payloads are stored.
// A completed submission removes the record; ambiguous outcomes retain it.
func pendingTaskKey(endpoint, credential string, body []byte) (string, func() error, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", nil, err
	}
	dir := filepath.Join(base, "hvx", "pending-tasks")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", nil, err
	}
	scope := sha256.New()
	fmt.Fprintf(scope, "%d:%s%d:%s", len(endpoint), endpoint, len(credential), credential)
	scope.Write(body)
	path := filepath.Join(dir, hex.EncodeToString(scope.Sum(nil)))
	lock := flock.New(path + ".lock")
	if err := lock.Lock(); err != nil {
		return "", nil, err
	}
	defer lock.Close()
	var ownedKey string
	read := func() (string, error) {
		b, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		key := string(b)
		if len(key) != 64 {
			return "", fmt.Errorf("invalid pending task record; refusing to replace an uncertain operation")
		}
		if _, err := hex.DecodeString(key); err != nil {
			return "", err
		}
		return key, nil
	}
	cleanup := func() error {
		guard := flock.New(path + ".lock")
		if err := guard.Lock(); err != nil {
			return err
		}
		defer guard.Close()
		current, err := read()
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if current != ownedKey {
			return nil
		}
		if err := os.Remove(path); err != nil {
			return err
		}
		return nil
	}
	if key, err := read(); err == nil {
		ownedKey = key
		return key, cleanup, nil
	} else if !os.IsNotExist(err) {
		return "", nil, err
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", nil, err
	}
	key := hex.EncodeToString(random)
	// Publish a fully written record without overwriting another process's key.
	tmp, err := os.CreateTemp(dir, ".pending-*")
	if err != nil {
		return "", nil, err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.WriteString(key); err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err != nil {
		return "", nil, err
	}
	if closeErr != nil {
		return "", nil, closeErr
	}
	if err = os.Link(tmp.Name(), path); err != nil {
		if !os.IsExist(err) {
			return "", nil, err
		}
		key, err = read()
		if err != nil {
			return "", nil, err
		}
	}
	// Persist the directory entry before a request can reach the server.
	if runtime.GOOS != "windows" {
		d, err := os.Open(dir)
		if err != nil {
			return "", nil, err
		}
		err = d.Sync()
		d.Close()
		if err != nil {
			return "", nil, err
		}
	}

	ownedKey = key
	return key, cleanup, nil
}
