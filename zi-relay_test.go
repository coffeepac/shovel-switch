package main

import (
    ioutil "io/ioutil"
    "testing"
    http "net/http"
    "time"
    "os"
    "fmt"
)


func TestStatusServer(t *testing.T) {
    quitChan = make(chan bool, 1)
    chefStatusChan = make(chan bool, 1)
    go statusServer(quitChan, chefStatusChan)

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

    //  test quit
    _, err = http.Get("http://localhost:7003/quit")
    if err != nil {
        t.Fatalf("Failed to query /quit with %s", err)
    }

    select {
        case q := <-quitChan:
            if q {
                t.Log("Successfully quit!")
            } else {
                t.Error("Got a bad message.  Should only ever be true.  Physics:  broken")
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
    http.HandleFunc("/ZIOn", ziOnHandle)
    http.HandleFunc("/ZIOff", ziOffHandle)

    http.ListenAndServe(":7000", nil)
}

func ziOnHandle(w http.ResponseWriter, r *http.Request){
    fmt.Fprintf(w, "{\"connObjectList\":[{\"deviceType\":\"BATS\",\"deviceName\":\"10.151.1.151\",\"connected\":false}],\"usingBATS\":true,\"secondsUntilUserCanInteract\":0,\"connectionExplanation\":\"Ship in motion\",\"userOverride\":false}")
}

func ziOffHandle(w http.ResponseWriter, r *http.Request){
    fmt.Fprintf(w, "{\"connObjectList\":[{\"deviceType\":\"BATS\",\"deviceName\":\"10.151.1.151\",\"connected\":false}],\"usingBATS\":false,\"secondsUntilUserCanInteract\":0,\"connectionExplanation\":\"Ship in motion\",\"userOverride\":false}")
}

func TestShovelManagement(t *testing.T){
    dummyRabbit := "echo"
    rabbitProg = dummyRabbit

    ziStatusFeeds := make(map[string] chan bool, 2)
    ziStatusFeeds["shovel"] = make(chan bool, 10)
    ziStatusFeeds["chef"] = make(chan bool, 10)

    go dummyZI()
    time.Sleep(1 * time.Second)

    goodUri := "http://localhost:7000/ZIOn"
    go zeroImpactMonitor(&goodUri, ziStatusFeeds, true)
    go shovelManagement(ziStatusFeeds["shovel"], true)
    go chefClientExecutor(ziStatusFeeds["chef"], chefStatusChan, true)

    time.Sleep(1 * time.Second)

    badUri := "http://localhost:7000/ZIOff"
    go zeroImpactMonitor(&badUri, ziStatusFeeds, true)

    time.Sleep(1 * time.Second)
}
