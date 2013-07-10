package couchbasestore

import	("encoding/base32"
	"github.com/couchbaselabs/go-couchbase"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"net/http"
	"strings")


type CouchStore struct{
	Bucket *couchbase.Bucket
	Codecs []securecookie.Codec
	Options *sessions.Options
	Pool *couchbase.Pool
	BucketName string
}

func NewCouchStore(endpoint string, pool string, bucket string, path string,maxAge int,keyPairs ...[]byte) *CouchStore{
	c,_ := couchbase.Connect(endpoint)
	p,_ := c.GetPool(pool)
	return  &CouchStore{
		Bucket: nil,
		Codecs: securecookie.CodecsFromPairs(keyPairs...),
		Options:&sessions.Options{
			Path:path,
			MaxAge:maxAge,
		},
		Pool: &p,
		BucketName: bucket,
	}
}

func (c *CouchStore) getBucket() (*couchbase.Bucket){
	c.Bucket,_ = c.Pool.GetBucket(c.BucketName)
	return c.Bucket
}

func (c *CouchStore) closeBucket(){
	c.Bucket.Close()
}

func (c *CouchStore) Close(){
	c.Bucket.Close()
}

func (c *CouchStore) Get(r *http.Request,name string) (*sessions.Session,error){
	return sessions.GetRegistry(r).Get(c,name)
}

func (c *CouchStore) New(r *http.Request, name string)(*sessions.Session,error){
	session := sessions.NewSession(c,name)
	session.Options = &(*c.Options)
	session.IsNew = true
	var err error
	if cook, errCookie := r.Cookie(name); errCookie == nil {
		err = securecookie.DecodeMulti(name, cook.Value, &session.ID, c.Codecs...)
		if err == nil {
			err = c.load(session)
			if err == nil {
				session.IsNew = false
			}
		}
	}
	return session, err
}

func (c *CouchStore) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	// Build an alphanumeric key for the redis store.
	if session.ID == "" {
		session.ID = strings.TrimRight(base32.StdEncoding.EncodeToString(securecookie.GenerateRandomKey(32)), "=")
	}
	if err := c.save(session); err != nil {
		return err
	}
	encoded, err := securecookie.EncodeMulti(session.Name(), session.ID, c.Codecs...)
	if err != nil {
		return err
	}
	http.SetCookie(w, sessions.NewCookie(session.Name(), encoded, session.Options))
	return nil
}

func (c *CouchStore) Delete(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	c.getBucket()
	c.Bucket.Delete(session.ID)
	defer c.Bucket.Close()
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

func (c *CouchStore) save(session *sessions.Session) error{
	encoded, err := securecookie.EncodeMulti(session.Name(), session.Values, c.Codecs...)
	if err != nil {
		return err
	}
	c.getBucket()
	defer c.closeBucket()
	errSet := c.Bucket.Set(session.ID,c.Options.MaxAge,encoded)
	if errSet != nil{
		return errSet
	}
	return nil
}

func (c *CouchStore) load(session *sessions.Session) error{
	c.getBucket()
	defer c.closeBucket()
	var data interface{}
	err := c.Bucket.Get(session.ID,&data)
	if err!=nil{
		return nil
	}
	if data == nil{
		return nil
	}
	str := data.(string)
	if err = securecookie.DecodeMulti(session.Name(), str ,&session.Values, c.Codecs...); err != nil {
		return err
	}
	return nil

}