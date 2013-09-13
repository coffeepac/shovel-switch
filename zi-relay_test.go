package main

import (
    ioutil "io/ioutil"
    "testing"
    http "net/http"
    "time"
    "os"
    "fmt"
)

var (
    dummyZILatch = false
)


func TestStatusServer(t *testing.T) {
    quitChan = make(chan bool, 1)
    stopZIMon = make(chan bool, 10)
    cmdStatusReq = make(map[string] chan bool, 2)
    cmdStatusReq["shovel"] = make(chan bool)
    cmdStatusReq["chef"] = make(chan bool)

    cmdStatusResp = make(map[string] chan bool, 2)
    cmdStatusResp["shovel"] = make(chan bool)
    cmdStatusResp["chef"] = make(chan bool)
    go statusServer()

    //  let the other go routine get started
    time.Sleep(1 * time.Millisecond)

    //  test ping
    resp, err := http.Get("http://localhost:7003/ping")
    if err != nil {
        t.Fatalf("Failed to query /ping with %s", err)
    }

    body, err := ioutil.ReadAll(resp.Body)
    if string(body) != "PONG\n" {
        t.Errorf("Expected PONG, got %s", string(body))
    }

    cStatus := false
    go func(){
        for {
            <-cmdStatusReq["shovel"]
            cmdStatusResp["shovel"] <- cStatus
        }
    }()

    go func(){
        for {
            <-cmdStatusReq["chef"]
            cmdStatusResp["chef"] <- cStatus
        }
    }()

    checkQuit(true, t)

    cStatus = true
    checkQuit(false, t)

}

func checkQuit(qExp bool, t *testing.T) {
    _, err := http.Get("http://localhost:7003/quit")
    if err != nil {
        t.Fatalf("Failed to query /quit with %s", err)
    }

    select {
        case q := <-quitChan:
            if q == qExp {
                t.Log("Successfully called quit endpoint")
            } else {
                t.Errorf("Did not get expected response from quit.  Expected %s, got %s\n", qExp, q)
            }
        case <- time.After(1 * time.Second):
            t.Error("Failed to get a message back from quit handler after 1 second")
    }
}

func TestPidFile(t *testing.T) {
    pfile := "pidfile"
    pidfile = &pfile
    check_pidfile()
    stat, err := os.Stat(pfile)
    if err != nil {
        t.Fatalf("could not stat file! %s", err)
    }

    if stat != nil {
        t.Log("pidfile created successfully")
    } else {
        t.Error("pidfile note created")
    }

    remove_pidfile()
    stat, err = os.Stat(pfile)
    if err != nil {
        t.Logf("file does not exist. %s", err)
    } else {
        t.Error("pidfile was not removed.")
    }
}

func dummyZI(){
    if dummyZILatch {
        return
    } else {
        dummyZILatch = true
        http.HandleFunc("/ZIOn", ziOnHandle)
        http.HandleFunc("/ZIOff", ziOffHandle)

        http.ListenAndServe(":7000", nil)
    }
}

func ziOnHandle(w http.ResponseWriter, r *http.Request){
    fmt.Fprintf(w, "{\"connObjectList\":[{\"deviceType\":\"BATS\",\"deviceName\":\"10.151.1.151\",\"connected\":false}],\"usingBATS\":true,\"secondsUntilUserCanInteract\":0,\"connectionExplanation\":\"Ship in motion\",\"userOverride\":false}")
}

func ziOffHandle(w http.ResponseWriter, r *http.Request){
    fmt.Fprintf(w, "{\"connObjectList\":[{\"deviceType\":\"BATS\",\"deviceName\":\"10.151.1.151\",\"connected\":false}],\"usingBATS\":false,\"secondsUntilUserCanInteract\":0,\"connectionExplanation\":\"Ship in motion\",\"userOverride\":false}")
}

func TestShovelStartManagement(t *testing.T){
    rabbitProg = "./sleep-short.sh"
    chefClient = "./sleep-long.sh"
    testManagement("http://localhost:7000/ZIOn", "shovel", 4, 1, t)
}

func TestShovelStopManagement(t *testing.T){
    rabbitProg = "./sleep-short.sh"
    chefClient = "./sleep-long.sh"
    testManagement("http://localhost:7000/ZIOff", "shovel", 4, 1, t)
}

func TestChefClientManagment(t *testing.T) {
    rabbitProg = "./sleep-long.sh"
    chefClient = "./sleep-short.sh"
    testManagement("http://localhost:7000/ZIOn", "chef", 4, 3, t)
}

func testManagement(testUri, appName string, sleepOne, sleepTwo int, t *testing.T) {
    ziStatusFeeds := make(map[string] chan bool, 2)
    ziStatusFeeds["shovel"] = make(chan bool, 10)
    ziStatusFeeds["chef"] = make(chan bool, 10)

    cmdStatusReq = make(map[string] chan bool, 2)
    cmdStatusReq["shovel"] = make(chan bool)
    cmdStatusReq["chef"] = make(chan bool)

    cmdStatusResp = make(map[string] chan bool, 2)
    cmdStatusResp["shovel"] = make(chan bool)
    cmdStatusResp["chef"] = make(chan bool)

    stopZIMon = make(chan bool, 1)

    go dummyZI()
    time.Sleep(1 * time.Second)

    go zeroImpactMonitor(&testUri, ziStatusFeeds, true)
    go shovelManagement(ziStatusFeeds["shovel"], cmdStatusReq["shovel"], cmdStatusResp["shovel"], 2, true)
    go ciManagement("chef-sleep-client", ziStatusFeeds["chef"], cmdStatusReq["chef"], cmdStatusResp["chef"], chefClientAction, 2, true)

    //  lets all go routines start
    time.Sleep(time.Duration(sleepOne) * time.Second)

    cmdStatusReq[appName] <- true
    appNameStatus := <-cmdStatusResp[appName]

    if appNameStatus {
        t.Log(appName + " command is running as expected")
    } else {
        t.Error(appName + " command is not reporting it is running.  It should be")
    }

    //  wait for the short sleep to finish
    time.Sleep(time.Duration(sleepTwo) * time.Second)

    //  shove a sleep in to make sure we don't grab our own message
    cmdStatusReq[appName] <- true
    appNameStatus = <-cmdStatusResp[appName]

    if !appNameStatus {
        t.Log(appName + " command is not running as expected")
    } else {
        t.Error(appName + " command is reporting it is running.  It should not be")
    }

    stopZIMon <- false
    ziStatusFeeds = nil
    cmdStatusReq = nil
    cmdStatusResp = nil
}
