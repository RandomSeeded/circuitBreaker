package main

import "fmt"
import "encoding/json"
// import "io/ioutil"
import "os"
import "net/http"
import "net/http/httputil"
import "net/url"

type Configuration struct {
  ListenPort  string
  RedirectPort  string
  RedirectHost  string
}

func readConfig() (Configuration, error) {
  configuration := Configuration{}

  configurationFile, openErr := os.Open("./config/config.json")
  if openErr != nil {
    return configuration, openErr
  }
  defer configurationFile.Close()

  decoder := json.NewDecoder(configurationFile)
  decodeErr := decoder.Decode(&configuration)
  if decodeErr != nil {
    return configuration, decodeErr
  }
  
  return configuration, nil
}

var numSuccesses = 0
var numFailures = 0
func TrackSuccess(_ *http.Response) (err error) {
  numSuccesses++
  return nil
}
func TrackFailure(_ http.ResponseWriter, _ *http.Request, _ error) {
  numFailures++
}

func serveReverseProxy(target string, res http.ResponseWriter, req *http.Request) {
  url, parseErr := url.Parse(target)
  if parseErr != nil {
    fmt.Printf("%v", parseErr)
  }

  // Reverse proxy
  proxy := httputil.NewSingleHostReverseProxy(url)
  proxy.ModifyResponse = TrackSuccess
  proxy.ErrorHandler = TrackFailure

  // Set headers (ssl forwarding)
  req.URL.Host = url.Host
  req.URL.Scheme = url.Scheme
  req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
  req.Host = url.Host

  proxy.ServeHTTP(res, req)
}

func main() {
  config, err := readConfig()
  if err != nil {
    fmt.Printf("Invalid configuration configurationFile provided %v\n", err)
    os.Exit(1)
  }

  http.HandleFunc("/", func (res http.ResponseWriter, req *http.Request) {
    fmt.Printf("successes %v failures %v\n", numSuccesses, numFailures)
    serveReverseProxy("http://localhost:8082", res, req)
  })

  fmt.Printf("Listening on port %v", config.ListenPort)
  http.ListenAndServe(config.ListenPort, nil)
}
