package main

import "fmt"
import "encoding/json"
// import "io/ioutil"
import "os"
import "net/http"
import "net/http/httputil"
import "net/url"
import "strconv"

type Configuration struct {
  ListenPort  string
  RedirectPort  string
  RedirectHost  string
  FailureRatio  string
  LookbackPeriod  int
}

var config Configuration
func readConfig() (Configuration, error) {
  if config != (Configuration{}) {
    return config, nil
  }

  config = Configuration{}

  configurationFile, openErr := os.Open("./config/config.json")
  if openErr != nil {
    return config, openErr
  }
  defer configurationFile.Close()

  decoder := json.NewDecoder(configurationFile)
  decodeErr := decoder.Decode(&config)
  if decodeErr != nil {
    return config, decodeErr
  }
  
  return config, nil
}

var numSuccesses = 0
var numFailures = 0
var responses = make([]string, 0)

func flushQueue(queue []string) (flushedQueue []string) {
  config, _ := readConfig()

  if len(queue) <= config.LookbackPeriod {
    return queue
  }

  oldestResponse := queue[0]
  queue = queue[1:]
  if oldestResponse == "success" {
    numSuccesses--
  } else {
    numFailures--
  }
  return queue
}

func TrackSuccess(_ *http.Response) (err error) {
  responses = append(responses, "success")
  numSuccesses++
  responses = flushQueue(responses)
  return nil
}

func TrackFailure(_ http.ResponseWriter, _ *http.Request, _ error) {
  responses = append(responses, "failure")
  numFailures++
  responses = flushQueue(responses)
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

// TODO (nw): get clarification on what best practices are for getting the config object everywhere are
func makeRequestHandler(config Configuration) func(http.ResponseWriter, *http.Request) {
  return func (res http.ResponseWriter, req *http.Request) {
    failureRatio, _ := strconv.ParseFloat(config.FailureRatio, 32)

    fmt.Printf("successes %v failures %v responses %v\n", numSuccesses, numFailures, responses)
    if len(responses) == config.LookbackPeriod && float64(numSuccesses) / float64(numSuccesses + numFailures) < failureRatio {
      fmt.Printf("Circuit break!\n")
      // TODO (nw): put the circuit break logic here
    }
    serveReverseProxy("http://localhost:8082", res, req)
  }
}

func main() {
  config, err := readConfig()
  fmt.Printf("config %v\n", config)
  if err != nil {
    fmt.Printf("Invalid configuration configurationFile provided %v\n", err)
    os.Exit(1)
  }

  http.HandleFunc("/", makeRequestHandler(config))

  fmt.Printf("Listening on port %v\n", config.ListenPort)
  http.ListenAndServe(config.ListenPort, nil)
}
