package main

import (
  "os"
  "flag"
  "log"
  "time"
  "fmt"
  "strconv"
  json "encoding/json"
  http "net/http"
)

var (
    uri          = flag.String("uri", "http://zeroimpact.mtnsatcloud.com:8084/v1.0/connectionStatus/", "ZeroImpact URI")
    pidfile      = flag.String("pidfile", "", "optional, write pid of self to here")
    healthport   = flag.Int("healthport", 7003, "port to listen for ping/quit requests on")
    shipcode     = flag.String("shipcode", "UNKNOWN", "shipcode to use as lookup into chef-server for jenkins promote job")
    verbose      = flag.Bool("verbose", false, "increase logging output")
    configfile   = flag.String("config", "", "full path to config file")
)

var (
    flowdcommand = "sv"
    rabbitProg  = "/etc/init.d/rabbitmq-stopable-shovel"
    chefClient  = "chef-client"
    quitChan    chan bool
    stopZIMon   chan bool
    quitFeeds   map[string] chan bool
    cmdStatusReq    map[string] chan bool
    cmdStatusResp   map[string] chan bool
)

type zeroimpactResponse struct {
    UsingBats bool `json:"usingBats"`
}

/*
**  serve status/health requests
*   kill application when told to
*/
func statusServer() {
    http.HandleFunc("/ping", pingHandle)
    http.HandleFunc("/quit", quitHandle)

    //  create server that doesn't leave things open forever
    s := &http.Server{
            Addr:           ":7003",
            ReadTimeout:    10 * time.Second,
            WriteTimeout:   10 * time.Second,
        }
    s.ListenAndServe()
}

func pingHandle(w http.ResponseWriter, r *http.Request){
    fmt.Fprintf(w, "PONG\n")
}

func quitHandle(w http.ResponseWriter, r *http.Request) {
    //  check if a chef-client run is on-going
    canStop := false
    for name, sChan := range cmdStatusReq {
        sChan <- true
        cStatus := <-cmdStatusResp[name]
        canStop = canStop || cStatus
    }
    if canStop {
        fmt.Fprintf(w, "one or more external commands are running.  Please wait a few minutes and try again")
        quitChan <- false
    } else {
        fmt.Fprintf(w, "zi-relay is now shutting down\n")
        stopZIMon <- false
        quitChan <- true
        for _, qChan := range quitFeeds {
            qChan <- true
        }
    }
}

/*
**  check_pidfile - if pidfile flag is set, write pid to it
*/
func check_pidfile(){
    if *pidfile != "" {
        pid := []byte(strconv.Itoa(os.Getpid()))
        pfile, err := os.Create(*pidfile)
        if err != nil {
            log.Println("Could not open pidfile: " + *pidfile + ".  Carrying on")
        } else {
            pfile.Write(pid)
            pfile.Close()
        }
    }
}

/*
**  remove_pidfile - if pidfile flag is set, remove when shutting down
*/
func remove_pidfile(){
    if *pidfile != "" {
        err := os.Remove(*pidfile)
        if err != nil {
            log.Println("Could not remove pidfile:  " + *pidfile + ". With error: " + err.Error())
        }
    }
}



/*
**  zeroImpactMonitor - polls the zero impact status interface and notifies
**                      all chans in feed map of current status
**
*/
func zeroImpactMonitor(uri *string, feeds map[string] chan bool, verbose bool) {
    //  poll the zi status interface until told to stop
    monitor := true
    go func(){
        for {
            monitor = <-stopZIMon
        }
    }()

    for monitor {
        resp, err := http.Get(*uri)
        if err != nil {
            log.Printf("Failed to access ZeroImpact service at %s with error %s\n", *uri, err)
        } else {
            var ziStatus zeroimpactResponse
            decoder := json.NewDecoder(resp.Body)
            err = decoder.Decode(&ziStatus)
            resp.Body.Close()
            if err != nil {
                log.Printf("failed to decode zi response, %s\n", err)
            } else {
                for _, feed := range feeds {
                    feed <- ziStatus.UsingBats
                }
            }
        }
        time.Sleep(5 * time.Second)
    }
}

/*
**  main - handles creation of main go routines
**       - flag parsing
**       - server creation
**       - pidfile handling
**       - zi checker
*/
func main(){
    flag.Parse()
    check_pidfile()
    defer remove_pidfile()

    ziStatusFeeds := make(map[string] chan bool, 2)
    ziStatusFeeds["tbn"] = make(chan bool, 10)
    ziStatusFeeds["vsat"] = make(chan bool, 10)
    quitFeeds = make(map[string] chan bool, 2)
    quitFeeds["tbn"] = make(chan bool)
    quitFeeds["vsat"] = make(chan bool)
    cmdStatusReq = make(map[string] chan bool, 2)
    cmdStatusReq["tbn"] = make(chan bool)
    cmdStatusReq["vsat"] = make(chan bool)
    cmdStatusResp = make(map[string] chan bool, 2)
    cmdStatusResp["tbn"] = make(chan bool)
    cmdStatusResp["vsat"] = make(chan bool)
    go zeroImpactMonitor(uri, ziStatusFeeds, *verbose)

    //  manage the TBN flowd
    go flowdManagement(ziStatusFeeds["tbn"], quitFeeds["tbn"], cmdStatusReq["tbn"], cmdStatusResp["tbn"], 5, "mtn-tbn", "tbn", *verbose)

    //  manage the harris vsat flowd
    go flowdManagement(ziStatusFeeds["vsat"], quitFeeds["vsat"], cmdStatusReq["vsat"], cmdStatusResp["vsat"], 5, "harris-vsat", "vsat",*verbose)

    //  status Server also handles quiting
    quitChan = make(chan bool)
    go statusServer()

    //  block until quitting time
    quit := false
    for !quit {
        quit = <-quitChan
    }
}

