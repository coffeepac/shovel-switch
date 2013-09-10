package main

import (
  "os"
  "flag"
  "log"
  "time"
  "fmt"
  "bytes"
  "strconv"
  json "encoding/json"
  http "net/http"
  exec "os/exec"
)

var (
    uri          = flag.String("uri", "http://zeroimpact.mtnsatcloud.com:8084/v1.0/connectionStatus/", "ZeroImpact URI")
    pidfile      = flag.String("pidfile", "", "optional, write pid of self to here")
    healthport   = flag.Int("healthport", 7003, "port to listen for ping/quit requests on")
    verbose      = flag.Bool("verbose", false, "increase logging output")
)

var (
    rabbitProg   = "/etc/init.d/rabbitmq-stopable-shovel"
    chefClient   = "chef-client"
    quitChan chan bool
    chefStatusChan chan bool
)

type zeroimpactResponse struct {
    UsingBats bool `json:"usingBats"`
}

/*
**  serve status/health requests
*   kill application when told to
*/
func statusServer(quitChan chan bool, chefStatusChan chan bool) {
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
    fmt.Fprintf(w, "zi-relay is now shutting down\n")
    quitChan <- true
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


//  turn stopable shovel on or off
func shovelManagement(ziStatus chan bool, verbose bool) {
    //  verified that stopping a stoped shovel or starting a started shovel doesn't
    //  effect the rabbit broker.  the rabbit broker informs the caller that the 
    //  current state matches desires state and to go away.  it says 'err' but that's
    //  a gentle way of saying, 'YES!  AND I AM ALREADY!'
    for {
        usingBats := <-ziStatus
        command := "stop"
        if usingBats {
            command = "start"
        }
        if verbose {
            log.Println(rabbitProg + " " + command)
        }
        cmd := exec.Command(rabbitProg, command)
        var out bytes.Buffer
        cmd.Stdout = &out
        cmd.Stderr = &out
        err := cmd.Run()
        if err != nil {
            log.Printf(command + " command failed with: %s", err)
            log.Printf("outputs were: %s", out)
        }
    }
}

/*
**  zeroImpactMonitor - polls the zero impact status interface and notifies
**                      all chans in feed map of current status
**
*/
func zeroImpactMonitor(uri *string, feeds map[string] chan bool, verbose bool) {
    //  poll the zi status interface forever
    for {
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


//  starts the chef-client run and selects across feed and status
//  if feed is true and there isn't a chef-client running, start one
//    if chef-client is running, ignore whatever comes in on feed
//  starts a go routine that shares state (chefStatus) with this.  it
//  will return chefStatus on status whenever it is called
func chefClientExecutor(feed chan bool, status chan bool, verbose bool){
    chefStatus := false
    go func(){
        <-status
        status <- chefStatus
    }()

    cmdDone := make(chan error)
    var cmd *exec.Cmd
    var out bytes.Buffer
    for {
        select {
            case start := <-feed:
                //  start a chef-client if one isn't running
                if start && !chefStatus {
                    chefStatus = true
                    cmd = exec.Command(chefClient)
                    cmd.Stdout = &out
                    cmd.Stderr = &out
                    go func(){
                        cmdDone <- cmd.Run()
                    }()
                } //  else is a no-op
            case err := <-cmdDone:
                chefStatus = false
                if err != nil{
                    log.Printf(chefClient + " command failed with: %s", err)
                    log.Printf("outputs were: %s", out)
                }
                cmd = nil

        }
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
    ziStatusFeeds["shovel"] = make(chan bool, 10)
    ziStatusFeeds["chef"] = make(chan bool, 10)
    go zeroImpactMonitor(uri, ziStatusFeeds, *verbose)

    //  manage the stopable shovel
    go shovelManagement(ziStatusFeeds["shovel"], *verbose)

    //  manage the chef-client runs
    chefStatusChan = make(chan bool)
    go chefClientExecutor(ziStatusFeeds["chef"], chefStatusChan, *verbose)

    //  status Server also handles quiting
    quitChan = make(chan bool)
    go statusServer(quitChan, chefStatusChan)

    //  block until quitting time
    <-quitChan
}

