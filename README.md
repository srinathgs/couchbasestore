couchbasestore
==============

Gorilla's Session store implementation with Couchbase backend.

Gorilla's Sessions and their sessions store interface can be found [here](https://github.com/gorilla/sessions)


Installation
----------

Install this package as you would usually install any Go package.

Run `go get github.com/srinathgs/couchbasestore` from terminal. It gets installed in $GOPATH


Example
--------
    
    package main
    import (
        "fmt"
        "net/http"
        "github.com/srinathgs/couchbasestore"
        "github.com/gorilla/mux"
    )
    
    var store = couchbasestore.NewCouchStore("http://[<username>]:[<password>]@<ip>:<port>","<poolname>",
                                              "<bucketname>","/",3600,[]byte("secret-key"))
    func foobar(w http.ResponseWriter, r *http.Request){
      session,err := store.Get(r,"foo")
      defer session.Save(r,w)
      session.Values["bar"] = "baz"
      fmt.Fprintf(w,"<h1>You have successfully accessed sessions.</h1>")
    }
    
    func main(){
      r := mux.NewRouter()
      r.HandleFunc("/foo/{bar}",foobar)
      http.Handle("/",r)
      http.ListenAndServe(":8081",nil)
    }
