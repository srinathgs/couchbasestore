//This Software is licensed under MIT license available under the LICENSE File
package couchbasestore

import (
	"encoding/base32"
	"errors"
	"github.com/couchbaselabs/go-couchbase"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"net/http"
	"strings"
)

//CouchStore Definition

type CouchStore struct {
	endpoint   string
	bucket     *couchbase.Bucket
	codecs     []securecookie.Codec
	options    *sessions.Options
	pool       *couchbase.Pool
	bucketName string
}

//No. Of Retries. The program that uses this package can change this.
var Retries = 5

var ErrMaxAge = errors.New("Max age should be greater than zero")

//Create a new CouchStore.
func NewCouchStore(endpoint string, pool string, bucket string, path string, maxAge int, keyPairs ...[]byte) (*CouchStore, error) {
	c, err := couchbase.Connect(endpoint)
	if err != nil {
		return nil, err
	}
	p, err := c.GetPool(pool)
	if err != nil {
		return nil, err
	}
	if path == "" {
		path = "/"
	}
	if maxAge <= 0 {
		return nil, ErrMaxAge
	}
	return &CouchStore{
		endpoint: endpoint,
		bucket:   nil,
		codecs:   securecookie.CodecsFromPairs(keyPairs...),
		options: &sessions.Options{
			Path:   path,
			MaxAge: maxAge,
		},
		pool:       &p,
		bucketName: bucket,
	}, nil
}

//Internal Function to get a bucket from a pool
func (c *CouchStore) getBucket() *couchbase.Bucket {
	if c.bucket == nil {
		c.bucket, _ = c.pool.GetBucket(c.bucketName)
	}
	return c.bucket
}

//Internal Function to close couchbase bucket
func (c *CouchStore) closeBucket() {
	c.bucket.Close()
}

//Close CouchStore
func (c *CouchStore) Close() {
	c.closeBucket()
}

//Get Session data from CouchStore
func (c *CouchStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	return sessions.GetRegistry(r).Get(c, name)
}

//Create New Session. CouchStore.Get will perform this in case there is no existing session.
func (c *CouchStore) New(r *http.Request, name string) (*sessions.Session, error) {
	session := sessions.NewSession(c, name)
	//default values
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

//Save Session
func (c *CouchStore) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	// Build an alphanumeric key for the couchbase store.
	if session.ID == "" {
		session.ID = strings.TrimRight(base32.StdEncoding.EncodeToString(securecookie.GenerateRandomKey(32)), "=")
	}
	if err := c.save(session); err != nil {
		return err
	}
	encoded, err := securecookie.EncodeMulti(session.Name(), session.ID, c.codecs...)
	if err != nil {
		return err
	}
	http.SetCookie(w, sessions.NewCookie(session.Name(), encoded, session.Options))
	return nil
}

//Retry
func mayBeRetry(f func() error) error {
	var tries = 0
	var err error
	for tries < Retries {
		err = f()
		if err == couchbase.TimeoutError || err == couchbase.ErrTimeout {
			tries++
		} else if err == nil {
			return nil
		} else {
			return err
		}
	}
	return err
}

//Delete session from CouchStore and HTTP cookie
func (c *CouchStore) Delete(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	c.getBucket()
	_ = mayBeRetry(func() error {
		return c.bucket.Delete(session.ID)
	})

	//defer c.closeBucket()
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

//Save session to Couchbase
func (c *CouchStore) save(session *sessions.Session) error {
	encoded, err := securecookie.EncodeMulti(session.Name(), session.Values, c.codecs...)
	if err != nil {
		return err
	}
	c.getBucket()
	//defer c.closeBucket()
	errSet := mayBeRetry(func() error {
		return c.bucket.Set(session.ID, session.Options.MaxAge, encoded)
	})
	if errSet != nil {
		return errSet
	}
	return nil
}

//Load Session from couchbase
func (c *CouchStore) load(session *sessions.Session) error {
	c.getBucket()
	//defer c.closeBucket()
	var data interface{}
	err := mayBeRetry(func() error {
		return c.bucket.Get(session.ID, &data)
	})
	if err != nil {
		return nil
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
