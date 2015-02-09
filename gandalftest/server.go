// Copyright 2015 gandalf authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package gandalftest provides a fake implementation of the Gandalf API.
package gandalftest

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/gorilla/pat"
)

type user struct {
	Name string
	Keys map[string]string
}

type key struct {
	Name string
	Body string
}

// GandalfServer is a fake gandalf server. An instance of the client can be
// pointed to the address generated for this server
type GandalfServer struct {
	listener  net.Listener
	muxer     *pat.Router
	users     []string
	keys      map[string][]key
	usersLock sync.Mutex
}

// NewServer returns an instance of the test server, bound to the specified
// address. To get a random port, users can specify the :0 port.
//
// Examples:
//
//     server, err := NewServer("127.0.0.1:8080") // will bind on port 8080
//     server, err := NewServer("127.0.0.1:0") // will get a random available port
func NewServer(bind string) (*GandalfServer, error) {
	listener, err := net.Listen("tcp", bind)
	if err != nil {
		return nil, err
	}
	server := GandalfServer{
		listener: listener,
		keys:     make(map[string][]key),
	}
	server.buildMuxer()
	go http.Serve(listener, &server)
	return &server, nil
}

// Stop stops the server, cleaning the internal listener and freeing the
// allocated port.
func (s *GandalfServer) Stop() error {
	return s.listener.Close()
}

// URL returns the URL of the server, in the format "http://<host>:<port>/".
func (s *GandalfServer) URL() string {
	return fmt.Sprintf("http://%s/", s.listener.Addr())
}

// ServeHTTP handler HTTP requests, dealing with prepared failures before
// dispatching the request to the proper internal handler.
func (s *GandalfServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.muxer.ServeHTTP(w, r)
}

func (s *GandalfServer) buildMuxer() {
	s.muxer = pat.New()
	s.muxer.Post("/user", http.HandlerFunc(s.createUser))
}

func (s *GandalfServer) createUser(w http.ResponseWriter, r *http.Request) {
	s.usersLock.Lock()
	defer s.usersLock.Unlock()
	defer r.Body.Close()
	var usr user
	err := json.NewDecoder(r.Body).Decode(&usr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.users = append(s.users, usr.Name)
	keys := make([]key, 0, len(usr.Keys))
	for name, body := range usr.Keys {
		keys = append(keys, key{Name: name, Body: body})
	}
	s.keys[usr.Name] = keys
}
