package main

import (
  "log"
  "bytes"
  "time"
  exec "os/exec"
)

//  turn stopable shovel on or off
func flowdManagement(feed, quit, statusReq, statusResp chan bool, sleepSeconds int, flowdname, linktype string, verbose bool) {
    //  asynchronously report is chef running status
    flowdRunningStatus := false
    go func(){
        for {
            <-statusReq
            statusResp <- flowdRunningStatus
        }
    }()

    //  asynchronously set boolean for which rabbit command to run
    feedStatus := false
    go func(){
        for {
            feedStatus = <-feed
        }
    }()

    quitStatus := false
    go func(){
        for {
            quitStatus = <-quit
        }
    }()

    for !quitStatus {
        flowdRunningStatus = true
        command := "down"
        log.Printf("feedstatus is %t and linktype is %s\n", feedStatus, linktype)
        switch {
        case feedStatus && (linktype == "tbn"):
            command = "up"
        case !feedStatus && (linktype == "vsat"):
            command = "up"
        }
        if verbose {
            log.Println(flowdcommand + " " + command + " " + flowdname)
        }
        cmd := exec.Command(flowdcommand, command, flowdname)
        var out bytes.Buffer
        cmd.Stdout = &out
        cmd.Stderr = &out
        err := cmd.Run()
        if err != nil {
            handle_cmd_error("flowd " + flowdname + " failed with command: " + command, err, out)
        }
        flowdRunningStatus = false

        time.Sleep(time.Duration(sleepSeconds) * time.Second)
    }
}

//  turn stopable shovel on or off
func shovelManagement(feed, statusReq, statusResp chan bool, sleepSeconds int, verbose bool) {
    //  asynchronously report is chef running status
    shovelRunningStatus := false
    go func(){
        for {
            <-statusReq
            statusResp <- shovelRunningStatus
        }
    }()

    //  asynchronously set boolean for which rabbit command to run
    feedStatus := false
    go func(){
        for {
            feedStatus = <-feed
        }
    }()

    //  verified that stopping a stoped shovel or starting a started shovel doesn't
    //  effect the rabbit broker.  the rabbit broker informs the caller that the 
    //  current state matches desires state and to go away.  it says 'err' but that's
    //  a gentle way of saying, 'YES!  AND I AM ALREADY!'
    for {
        shovelRunningStatus = true
        command := "stop"
        if feedStatus {
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
            handle_cmd_error("stopable shovel", err, out)
        }
        shovelRunningStatus = false

        time.Sleep(time.Duration(sleepSeconds) * time.Second)
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
func ciManagement(name string, feed, statusReq, statusResp chan bool, action ciAction, sleepSeconds int, verbose bool){
    //  asynchronously report is chef running status
    chefStatus := false
    go func(){
        for {
            <-statusReq
            statusResp <- chefStatus
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
        handle_cmd_error("chef client", err, out)
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
        log.Printf("Failed to start promotion job with error: %s\n", err)
        return err
    }
    err = promote.Wait(1)
    if err != nil {
        log.Printf("Failed to wait for promotion job with error: %s\n", err)
    }
    return err
}

/*
**  handle_cmd_error - function to handle errors in command line executions
**                   only prints stdout and stderr to stdout for now, will 
**                   do more later
**
*/
func handle_cmd_error(name string, err error, out bytes.Buffer) {
    log.Printf(name + " command failed with: %s", err)
    log.Printf("outputs were: %s", out)
}
