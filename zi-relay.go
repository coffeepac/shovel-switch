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
    shipcode     = flag.String("shipcode", "UNKNOWN", "shipcode to use as lookup into chef-server for jenkins promote job")
    verbose      = flag.Bool("verbose", false, "increase logging output")
)

var (
    rabbitProg  = "/etc/init.d/rabbitmq-stopable-shovel"
    chefClient  = "chef-client"
    quitChan    chan bool
    stopZIMon   chan bool
    cmdStatus   map[string] chan bool
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
    for _, sChan := range cmdStatus {
        sChan <- true
        cStatus := <-sChan
        canStop = canStop || cStatus
    }
    if canStop {
        fmt.Fprintf(w, "one or more external commands are running.  Please wait a few minutes and try again")
        quitChan <- false
    } else {
        fmt.Fprintf(w, "zi-relay is now shutting down\n")
        stopZIMon <- false
        quitChan <- true
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

//  turn stopable shovel on or off
func shovelManagement(feed, status chan bool, verbose bool) {
    //  asynchronously report is chef running status
    shovelRunningStatus := false
    go func(){
        for {
            <-status
            status <- shovelRunningStatus
        }
    }()
    //  verified that stopping a stoped shovel or starting a started shovel doesn't
    //  effect the rabbit broker.  the rabbit broker informs the caller that the 
    //  current state matches desires state and to go away.  it says 'err' but that's
    //  a gentle way of saying, 'YES!  AND I AM ALREADY!'
    for {
        usingBats := <-feed
        shovelRunningStatus = true
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
            handle_cmd_error(err, out)
        }
        shovelRunningStatus = false
    }
}

/*  
**  ciManagement - handles execution of ci pieces and coordination
**                       with other functions
**  listens on the feed channel in a go routine which sets the feedStatus
**  which lets the ci job start
**    if the ci job is running, ignore whatever comes in on feed
**  starts a go routine that shares state (ciStatus) with this.  it
**  will return ciStatus on status whenever it is called
**  takes a function which handles the interop with the ci command to run
**  and any error handling.  
**    returns an error
*/
type ciAction func(verbose bool) (err error)
func ciManagement(name string, feed, status chan bool, action ciAction, sleepSeconds int, verbose bool){
    //  asynchronously report is chef running status
    chefStatus := false
    go func(){
        for {
            <-status
            status <- chefStatus
        }
    }()

    //  asynchronously set boolean for 'should start another chef client run'
    feedStatus := false
    go func(){
        for {
            feedStatus = <-feed
        }
    }()

    for {
        if feedStatus {
            if verbose {
                log.Println(name + ": ZI is on, begin the job")
            }
            chefStatus = true
            err := action(verbose)
            if err != nil {
                log.Println(name + " action failed")
            }
            chefStatus = false
        } else if !feedStatus && verbose {
            log.Println(name + ": ZI is off.  Do nothing")
        }

        time.Sleep(time.Duration(sleepSeconds) * time.Second)
    }
}

/*
**  chefClientAction - wrapper function that holds the chef client ci action
**
*/
func chefClientAction(verbose bool) (err error) {
    cmd := exec.Command(chefClient)
    var out bytes.Buffer
    cmd.Stdout = &out
    cmd.Stderr = &out
    err = cmd.Run()
    if verbose {
        log.Println("Finished a chef-client run")
    }
    if err != nil {
        handle_cmd_error(err, out)
    }
    return err
}

/*
**  fetchCIArtifacts - wrapper function around the call to our lib for handling running
**               promote-to-ship
**
**  all of the REST calls will be in the the promote-to-ship wrapper lib
*/
func fetchCIArtifacts(verbose bool) (err error) {
    promote := &PromoteToShip{Shipcode: *shipcode}
    err = promote.Start()
    if err != nil {
        log.Println("Failed to start promotion job with error: %s", err)
        return err
    }
    err = promote.Wait(1)
    if err != nil {
        log.Println("Failed to wait for promotion job with error: %s", err)
    }
    return err
}

/*
**  handle_cmd_error - function to handle errors in command line executions
**                   only prints stdout and stderr to stdout for now, will 
**                   do more later
**
*/
func handle_cmd_error(err error, out bytes.Buffer) {
    log.Printf(chefClient + " command failed with: %s", err)
    log.Printf("outputs were: %s", out)
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

    ziStatusFeeds := make(map[string] chan bool, 3)
    ziStatusFeeds["shovel"] = make(chan bool, 10)
    ziStatusFeeds["chef"] = make(chan bool, 10)
    ziStatusFeeds["promote"] = make(chan bool, 10)
    cmdStatus = make(map[string] chan bool, 3)
    cmdStatus["shovel"] = make(chan bool)
    cmdStatus["chef"] = make(chan bool)
    cmdStatus["promote"] = make(chan bool)
    go zeroImpactMonitor(uri, ziStatusFeeds, *verbose)

    //  manage the stopable shovel
    go shovelManagement(ziStatusFeeds["shovel"], cmdStatus["shovel"], *verbose)

    //  manage the chef-client runs
    go ciManagement("chef-client", ziStatusFeeds["chef"], cmdStatus["chef"], chefClientAction, 1, *verbose)

    //  manage the promote-to-ship runs
    go ciManagement("promote-to-ship", ziStatusFeeds["promote"], cmdStatus["promote"], fetchCIArtifacts, 1, *verbose)

    //  status Server also handles quiting
    quitChan = make(chan bool)
    go statusServer()

    //  block until quitting time
    quit := false
    for !quit {
        quit = <-quitChan
    }
}

