[![GoDoc](https://godoc.org/github.com/drathier/couchbasestore?status.png)](http://godoc.org/github.com/drathier/couchbasestore)
couchbasestore
==============

Gorilla's Session store implementation with Couchbase backend.

Gorilla's Sessions and their sessions store interface can be found [here](https://github.com/gorilla/sessions)

Currently, this package supports storing sessions in only one bucket. In case your application demands to keep sessions in different buckets, implement a container that will have couchbasestore as the underlying struct.

Couchbasestore is NOT thread-safe.

This package is backwards compatible with the original package by Srinath.
The original can be found at https://github.com/srinathgs/couchbasestore

Installation
----------

Install this package as you would usually install any Go package.

Run `go get github.com/drathier/couchbasestore` from terminal. It gets installed in $GOPATH


Example
--------

    package main

    import (
        "fmt"
        "github.com/couchbase/go-couchbase"
        "github.com/drathier/couchbasestore"
        "github.com/gorilla/mux"
        "net/http"
        "os"
    )

    // ignoring error values
    var bucket, _ = couchbase.GetBucket(os.Getenv("COUCHBASE_URI"), "default", os.Getenv("COUCHBASE_BUCKET"))
    var store, _ = couchbasestore.NewCouchStoreBucket(bucket, "/", 3600, []byte("secret-key"))

    func foobar(w http.ResponseWriter, r *http.Request) {
        session, _ := store.Get(r, "foo") //name is the key against which a cookie is set in the HTTP header
        defer session.Save(r, w)
        session.Values["bar"] = "baz"
        fmt.Fprintf(w, "<h1>You have successfully accessed sessions.</h1>")
    }

    func main() {
        r := mux.NewRouter()
        r.HandleFunc("/foo/{bar}", foobar)
        http.Handle("/", r)
        http.ListenAndServe(":8080", nil)
    }
