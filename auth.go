package main

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"hash"
	"strings"
)

func hashPasswordSalt(hasher hash.Hash, password, salt []byte) []byte {
	hasher.Write(password)
	hasher.Write(salt)
	return hasher.Sum(nil)
}

func checkPassword(userPassword, password string) error {
	if !strings.HasPrefix(userPassword, "{") {
		return errors.New("Incorrect format")
	}

	parts := strings.SplitN(userPassword[1:], "}", 2)
	scheme, b64hashsalt := parts[0], parts[1]
	hashsalt, err := base64.StdEncoding.DecodeString(b64hashsalt)
	if err != nil {
		return errors.New("Unable to decode base64-encoded password")
	}

	var hasher hash.Hash
	if scheme == "SSHA256" {
		hasher = sha256.New()
	} else if scheme == "SSHA" {
		hasher = sha1.New()
	} else {
		return fmt.Errorf("Unsupported encoding '%s'", scheme)
	}

	passwordHash, salt := hashsalt[0:hasher.Size()], hashsalt[hasher.Size():]
	res := hashPasswordSalt(hasher, []byte(password), salt)
	if !bytes.Equal(res, passwordHash) {
		return errors.New("Invalid password")
	}
	return nil
}
