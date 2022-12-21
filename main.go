package main

import (
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"time"
)

func handleTunneling(w http.ResponseWriter, r *http.Request) {
	log.Print("new tcp connection", r.Method)
	destConn, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	}
	go transfer(destConn, clientConn)
	go transfer(clientConn, destConn)
}

func transfer(destination io.WriteCloser, source io.ReadCloser) {
	defer destination.Close()
	defer source.Close()
	io.Copy(destination, source)
}

func handleHTTP(w http.ResponseWriter, req *http.Request) {
	rr, err := http.NewRequest(req.Method, req.RequestURI, req.Body)
	if err != nil {
		log.Print(err)
		return
	}
	copyHeader(req.Header, rr.Header)

	// Create a client and query the target
	var transport http.Transport
	resp, err := transport.RoundTrip(rr)

	log.Printf("Resp-Headers: %v\n", resp.Header)

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	dH := w.Header()
	copyHeader(resp.Header, dH)
	dH.Add("Requested-Host", rr.Host)

	w.Write(body)
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func main() {
	server := &http.Server{
		Addr: ":8080",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				handleTunneling(w, r)
			} else {
				handleHTTP(w, r)
			}
		}),
		// Disable HTTP/2.
		//TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}
	log.Print("start server")
	log.Fatal(server.ListenAndServe())
}
