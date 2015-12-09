// Copyright 2013 Srinath. All rights Reserved.
// Copyright 2015 drathier. All rights Reserved.
// This Software is licensed under MIT license available in the LICENSE file.
//
// Couchbasestore is NOT thread-safe.
//
// This package is backwards compatible with the original package by Srinath.
// The original can be found at https://github.com/srinathgs/couchbasestore
//
// Package couchbasestore implements the Gorilla toolkit's sessions store for couchbase.
// Gorilla's Sessions and their sessions store interface can be found [here](https://github.com/gorilla/sessions)
package couchbasestore

import (
	"encoding/base32"
	"errors"
	"github.com/couchbase/go-couchbase"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"net/http"
	"strings"
	"time"
)

// CouchStore Definition
type CouchStore struct {
	bucket  *couchbase.Bucket
	codecs  []securecookie.Codec
	options *sessions.Options
}

// Number of retries. The program that uses this package can change this.
var Retries = 5

// Duration between retries. Increases exponentially with subsequent failed attempts.
// Retries stop after roughly 1/2 * Retries*(Retries+1) * BaseRetryDuration.
var BaseRetryDuration = 50 * time.Millisecond

// MaxAge error.
var ErrMaxAge = errors.New("max age should be greater than zero")

// New creates a CouchStore using an existing couchbase.Bucket.
func New(bucket *couchbase.Bucket, cookiePath string, cookieMaxAge int, keyPairs ...[]byte) (*CouchStore, error) {
	if cookiePath == "" {
		cookiePath = "/"
	}
	if cookieMaxAge <= 0 {
		return nil, ErrMaxAge
	}
	return &CouchStore{
		bucket: bucket,
		codecs: securecookie.CodecsFromPairs(keyPairs...),
		options: &sessions.Options{
			Path:   cookiePath,
			MaxAge: cookieMaxAge,
		},
	}, nil
}

// Backwards-compatible alias for New.
func NewCouchStore(endpoint string, pool string, bucket string, cookiePath string, cookieMaxAge int, keyPairs ...[]byte) (*CouchStore, error) {
	return NewFromURI(endpoint, pool, bucket, cookiePath, cookieMaxAge, keyPairs)
}

// NewFromURI creates a new CouchStore with a new couchbase.Bucket.
func NewFromURI(endpoint string, pool string, bucket string, cookiePath string, cookieMaxAge int, keyPairs ...[]byte) (*CouchStore, error) {
	bucket, err := couchbase.GetBucket(endpoint, pool, bucket)
	if err != nil {
		return err
	}

	return New(bucket, cookiePath, cookieMaxAge, keyPairs...)
}

// Close closes the bucket used by CouchStore.
func (c *CouchStore) Close() {
	c.bucket.Close()
}

// Get Session data from CouchStore. name is the key in the cookie against which the cookie string is set.
func (c *CouchStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	return sessions.GetRegistry(r).Get(c, name)
}

// New creates a new session. CouchStore.Get will perform this in case there is no existing session.
func (c *CouchStore) New(r *http.Request, name string) (*sessions.Session, error) {
	session := sessions.NewSession(c, name)
	// default values
	session.Options = &sessions.Options{
		Path:   c.options.Path,
		MaxAge: c.options.MaxAge,
	}
	session.IsNew = true
	var err error
	if cook, errCookie := r.Cookie(name); errCookie == nil {
		err = securecookie.DecodeMulti(name, cook.Value, &session.ID, c.codecs...)
		if err == nil {
			err = c.load(session)
			if err == nil {
				session.IsNew = false
			}
		}
	}
	return session, err
}

// Save saves the session to couchbase using bucket c.bucket.
func (c *CouchStore) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	// Build an alphanumeric key for the couchbase store.
	if session.ID == "" {
		session.ID = strings.TrimRight(base32.StdEncoding.EncodeToString(securecookie.GenerateRandomKey(32)), "=")
	}
	encoded, err := securecookie.EncodeMulti(session.Name(), session.ID, c.codecs...)
	if err != nil {
		return err
	}

	if err := c.save(session); err != nil {
		return err
	}
	http.SetCookie(w, sessions.NewCookie(session.Name(), encoded, session.Options))
	return nil
}

// MaybeRetry runs the supplied function f until it no longer returns a timeout error, at most Retries times.
// If it returns another error, it returns that error immediately.
// Retries is a global variable.
func maybeRetry(f func() error) error {
	var failedAttempts = 0
	var err error
	for failedAttempts < Retries {
		err = f()
		if err == couchbase.TimeoutError || err == couchbase.ErrTimeout {
			failedAttempts++
			time.Sleep((failedAttempts + 1) * BaseRetryDuration) // exponential backoff
		} else {
			return err // unknown or no error
		}
	}
	return err
}

// Delete deletes session from CouchStore and HTTP cookie stores.
func (c *CouchStore) Delete(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	err := maybeRetry(func() error {
		return c.bucket.Delete(session.ID)
	})
	if err != nil {
		return err
	}

	// Set cookie to expire.
	options := *session.Options
	options.MaxAge = -1
	http.SetCookie(w, sessions.NewCookie(session.Name(), "", &options))
	// Clear session values.
	for k := range session.Values {
		delete(session.Values, k)
	}
	return nil
}

// Save saves the session to couchbase using bucket c.bucket.
func (c *CouchStore) save(session *sessions.Session) error {
	encoded, err := securecookie.EncodeMulti(session.Name(), session.Values, c.codecs...)
	if err != nil {
		return err
	}

	err = maybeRetry(func() error {
		return c.bucket.Set(session.ID, session.Options.MaxAge, encoded)
	})
	if err != nil {
		return err
	}
	return nil
}

// Load loads a session from couchbase using bucket c.bucket.
func (c *CouchStore) load(session *sessions.Session) error {
	var data interface{}
	err := maybeRetry(func() error {
		return c.bucket.Get(session.ID, &data)
	})
	if err != nil {
		return err
	}
	if data == nil {
		return nil
	}
	str := data.(string)
	if err = securecookie.DecodeMulti(session.Name(), str, &session.Values, c.codecs...); err != nil {
		return err
	}
	return nil

}
