package jenkins

import (
    "time"
    "fmt"
    "testing"
    http "net/http"
)

var (
    dummyJenkinsLatch = false
    jobStarted = false
    jobCompleted = false
    shipcode string
)

func dummyJenkins(){
    if dummyJenkinsLatch {
        return
    } else {
        dummyJenkinsLatch = true
        http.HandleFunc("/promote", promoteHandle)
        http.HandleFunc("/lastBuild", lastBuildHandle)
        http.HandleFunc("/jobResult", jobResultHandle)

        http.ListenAndServe(":7005", nil)
    }
}

func promoteHandle(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        w.WriteHeader(405)
        fmt.Fprintf(w, "Only accepts POST")
    } else {
        err := r.ParseForm()
        if err != nil {
            fmt.Printf("Failed to parse form data")
        } else {
            if sc := r.Form.Get("SHIPNAME"); sc != "" {
                w.WriteHeader(201)
            } else {
                w.WriteHeader(400)
                fmt.Fprintf(w, "Failed to include Shipcode form param")
            }
        }
    }
}

//  make sure you have something that flips the jobStarted value back and forth
func lastBuildHandle(w http.ResponseWriter, r *http.Request) {
    if r.Method != "GET" {
        w.WriteHeader(405)
        fmt.Fprintf(w, "Only accepts POST")
    } else {
        if !jobStarted {
            fmt.Fprintf(w, "{\"lastBuild\":{\"actions\":[{\"parameters\":[{\"name\":\"SHIPCODE\",\"value\":\"nachoShip\"}]},{}],\"url\":\"http://localhost:7005//\"}}")
        } else {
            fmt.Fprintf(w, "{\"lastBuild\":{\"actions\":[{\"parameters\":[{\"name\":\"SHIPCODE\",\"value\":\"" + shipcode + "\"}]},{}],\"url\":\"http://localhost:7005//\"}}")
        }
    }
}

func jobResultHandle(w http.ResponseWriter, r *http.Request) {
    if r.Method != "GET" {
        w.WriteHeader(405)
        fmt.Fprintf(w, "Only accepts POST")
    } else {
        if !jobCompleted {
            fmt.Fprintf(w, "{\"result\":null}")
        } else {
            fmt.Fprintf(w, "{\"result\":\"SUCCESS\"}")
        }
    }
}

func TestStartGood(t *testing.T) {
    go dummyJenkins()

    //  let server start
    time.Sleep(time.Second)

    ciPostURL = "http://localhost:7005/promote"
    local := &PromoteToShip{Shipcode: "local"}
    err := local.Start()
    if err != nil {
        t.Errorf("Failed start command with: %s", err)
    } else {
        t.Log("First postform a success!")
    }
}

func TestStartNoShipCode(t *testing.T) {
    go dummyJenkins()

    //  let server start
    time.Sleep(time.Second)

    ciPostURL = "http://localhost:7005/promote"
    local := &PromoteToShip{}
    err := local.Start()
    if err != nil {
        t.Logf("Correctly failed lack of shipcode with: %s", err)
    } else {
        t.Error("Succeeded when it should have failed from lack of shipcode!")
    }
}

func TestWaitGood(t *testing.T) {
    go dummyJenkins()

    time.Sleep(time.Second)
    ciLastBuild = "http://localhost:7005/lastBuild"
    ciResult = "jobResult"

    //  wait 5 seconds to flip from job not started to job started
    go func() {
        time.Sleep(5 * time.Second)
        jobStarted = true
    }()

    //  wait 10 seconds to flip from job not complete to job completed
    go func() {
        time.Sleep(10 * time.Second)
        jobCompleted = true
    }()

    shipcode = "localWait"
    local := &PromoteToShip{Shipcode: shipcode}
    err := local.Wait(1)

    if err != nil {
        t.Errorf("Wait call failed when it shouldn't have with: %s", err)
    } else {
        t.Log("Wait experiences great success!")
    }
}
