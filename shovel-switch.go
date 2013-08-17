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
  exec "os/exec"
)

var (
    uri          = flag.String("uri", "http://zeroimpact.mtnsatcloud.com:8084/v1.0/connectionStatus/", "ZeroImpact URI")
    rabbitProg   = flag.String("rabbitProg", "rabbitmqctl", "name of application to use to access rabbit.  only set if not rabbitmqctl or if rabbitmqctl is not in $PATH")
    rabbitName   = flag.String("rabbitName", "rabbitmq-stopable-shovel", "Node name of rabbit that has the stopable shovels")
    pidfile      = flag.String("pidfile", "", "optional, write pid of self to here")
    healthport   = flag.Int("healthport", 7003, "port to listen for ping/quit requests on")
)

var (
    quitChan chan bool
)

type zeroimpactResponse struct {
    usingBats bool `json:"usingBats"`
}

/*
**  serve status/health requests
*   kill application when told to
*/
func statusServer(quitChan chan bool) {
    log.Println("servin'")
    http.HandleFunc("/ping", pingHandle)
    http.HandleFunc("/quit", quitHandle)

    http.ListenAndServe(":7003", nil)
}

func pingHandle(w http.ResponseWriter, r *http.Request){
    fmt.Fprintf(w, "PONG\n")
}

func quitHandle(w http.ResponseWriter, r *http.Request) {
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

func shovelManagement(uri *string) {
    //  run forever!
    //   verified that stopping a stoped shovel or starting a started shovel doesn't
    //   effect the rabbit broker.  the rabbit broker informs the caller that the 
    //   current state matches desires state and to go away.  it says 'err' but that's
    //   a gentle way of saying, 'YES!  AND I AM ALREADY!'
    for {
        resp, err := http.Get(*uri)
        if err != nil {
            log.Printf("Failed to access ZeroImpact service at %s with error %s\n", *uri, err)
        } else {
            var ziStatus zeroimpactResponse
            decoder := json.NewDecoder(resp.Body)
            err = decoder.Decode(&ziStatus)
            if err != nil {
                log.Printf("failed to decode zi response\n")
            } else {
                if ziStatus.usingBats {
                    cmd := exec.Command(*rabbitProg, "-n " + *rabbitName, "eval", "'application:start(rabbitmq_shovel).'")
                    err = cmd.Run()
                    if err != nil {
                        log.Printf("start command failed with: %s", err)
                    }
                } else {
                    cmd := exec.Command(*rabbitProg, "-n " + *rabbitName, "eval", "'application:stop(rabbitmq_shovel).'")
                    err = cmd.Run()
                    if err != nil {
                        log.Printf("stop command failed with: %s", err)
                    }
                }
            }
        }

        <-time.After(5 * time.Second)
    }
}


/*
**  main - handles creation of main go routines
**       - flag parsing
**       - server creation
*/
func main(){
    flag.Parse()
    check_pidfile()

    //  manage the stopable shovel
    go shovelManagement(uri)

    //  status Server also handles quiting
    quitChan = make(chan bool)
    go statusServer(quitChan)

    //  block until quitting time
    <-quitChan

}

