package lb

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

type myTransport struct {
	// Uncomment this if you want to capture the transport
	// CapturedTransport http.RoundTripper
}
type Montioringpath struct {
	Path        string
	Count       int64
	Duration    int64
	AverageTime int64
}

var globalMap = make(map[string]Montioringpath)

func (t *myTransport) RoundTrip(request *http.Request) (*http.Response, error) {

	fmt.Println("---------------------------New Request--------------------------------------------------")
	buf, _ := ioutil.ReadAll(request.Body)
	rdr1 := ioutil.NopCloser(bytes.NewBuffer(buf))
	rdr2 := ioutil.NopCloser(bytes.NewBuffer(buf))

	fmt.Println("Request body : ", rdr1)
	request.Body = rdr2 // OK since rdr2 implements the

	start := time.Now()
	response, err := http.DefaultTransport.RoundTrip(request)
	if err != nil {
		print("\n\ncame in error resp here", err)
		return nil, err //Server is not reachable. Server not working
	}
	elapsed := time.Since(start)

	key := request.Method + "-" + request.URL.Path //for example for POST Method with /path1 as url path key=POST-/path1

	if val, ok := globalMap[key]; ok {
		val.Count = val.Count + 1
		val.Duration += elapsed.Nanoseconds()
		val.AverageTime = val.Duration / val.Count
		globalMap[key] = val
		//do something here
	} else {
		var m Montioringpath
		m.Path = request.URL.Path
		m.Count = 1
		m.Duration = elapsed.Nanoseconds()
		m.AverageTime = m.Duration / m.Count
		globalMap[key] = m
	}
	b, err := json.MarshalIndent(globalMap, "", "  ")
	if err != nil {
		fmt.Println("error:", err)
	}

	body, err := httputil.DumpResponse(response, true)
	if err != nil {
		print("\n\nerror in dumb response")
		// copying the response body did not work
		return nil, err
	}

	log.Println("Response Body : ", string(body))
	log.Println("Response Time:", elapsed.Nanoseconds())

	fmt.Println("Analysis on different Path :", string(b))

	return response, err
}

type Prox struct {
	target *url.URL
	proxy  *httputil.ReverseProxy
}

func NewProxy(target string) *Prox {
	url, _ := url.Parse(target)

	return &Prox{target: url, proxy: httputil.NewSingleHostReverseProxy(url)}
}

func (p *Prox) handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-GoProxy", "GoProxy")
	p.proxy.Transport = &myTransport{}

	p.proxy.ServeHTTP(w, r)

}

var port *string
var redirecturl *string

func main() {
	const (
		defaultPort        = ":9090"
		defaultPortUsage   = "default server port, ':9090'"
		defaultTarget      = "http://127.0.0.1:8080"
		defaultTargetUsage = "default redirect url, 'http://127.0.0.1:8080'"
	)

	// flags
	port = flag.String("port", defaultPort, defaultPortUsage)
	redirecturl = flag.String("url", defaultTarget, defaultTargetUsage)

	flag.Parse()

	fmt.Println("server will run on :", *port)
	fmt.Println("redirecting to :", *redirecturl)

	// proxy
	proxy := NewProxy(*redirecturl)

	http.HandleFunc("/proxyServer", ProxyServer)

	// server redirection
	http.HandleFunc("/", proxy.handle)
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}

func ProxyServer(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Reverse proxy Server Running. Accepting at port:" + *port + " Redirecting to :" + *redirecturl))

}

func Init() {
	const (
		defaultPort        = ":9090"
		defaultPortUsage   = "default server port, ':9090'"
		defaultTarget      = "http://127.0.0.1:8080"
		defaultTargetUsage = "default redirect url, 'http://127.0.0.1:8080'"
	)

	// flags
	port = flag.String("port", defaultPort, defaultPortUsage)
	redirecturl = flag.String("url", defaultTarget, defaultTargetUsage)

	flag.Parse()

	fmt.Println("server will run on :", *port)
	fmt.Println("redirecting to :", *redirecturl)

	// proxy
	proxy := NewProxy(*redirecturl)

	http.HandleFunc("/proxyServer", ProxyServer)

	// server redirection
	http.HandleFunc("/", proxy.handle)
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}
