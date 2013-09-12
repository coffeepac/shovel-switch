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
    jobSuccess = true
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
        } else if jobSuccess {
            fmt.Fprintf(w, "{\"result\":\"SUCCESS\"}")
        } else {
            fmt.Fprintf(w, "{\"result\":\"FAILURE\"}")
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

func exerciseWait(localcode string, jobSuccessLocal bool) (local *PromoteToShip, err error){
    go dummyJenkins()

    time.Sleep(time.Second)
    ciPostURL = "http://localhost:7005/promote"
    ciLastBuild = "http://localhost:7005/lastBuild"
    ciResult = "jobResult"

    jobStarted = false
    jobCompleted = false

    //  wait 3 seconds to flip from job not started to job started
    go func() {
        time.Sleep(3 * time.Second)
        jobStarted = true
    }()

    //  wait 6 seconds to flip from job not complete to job completed
    jobSuccess = jobSuccessLocal
    go func() {
        time.Sleep(6 * time.Second)
        jobCompleted = true
    }()

    shipcode = localcode
    local = &PromoteToShip{Shipcode: shipcode}
    err = local.Start()
    if err != nil {
        return
    }
    err = local.Wait(1)
    return
}

func TestWaitSuccess(t *testing.T) {
    _, err := exerciseWait("localship", true)

    if err != nil {
        t.Errorf("Wait call failed when it shouldn't have with: %s", err)
    } else {
        t.Log("Wait experiences great success!")
    }
}

func TestWaitFailure(t *testing.T) {
    _, err := exerciseWait("localship", false)

    if err == nil {
        t.Error("Wait call succeeded when it shouldn't have with")
    } else {
        t.Logf("Wait correctly experienced failure with error: %s", err)
    }
}

func TestWaitNoShipCode(t *testing.T) {
    _, err := exerciseWait("", false)

    if err == nil {
        t.Error("Wait call succeeded when it shouldn't have with")
    } else {
        t.Logf("Wait correctly experienced failure with error: %s", err)
    }

}

func TestWaitNoStart(t *testing.T) {
    local := &PromoteToShip{Shipcode: shipcode}
    err := local.Wait(1)
    if err != nil {
        if err.Error() == "Must call Start before waiting for the job to finish" {
            t.Log("Correctly prevented Wait from executing if Start not called")
        } else {
            t.Errorf("Failed with unexpected error: %s", err)
        }
    } else {
        t.Error("Succeeded when it shouldn't have")
    }
}

func TestWaitDoubleWait(t *testing.T){
    local, errOrig := exerciseWait("localship", false)
    errRecall := local.Wait(1)

    if errOrig.Error() == errRecall.Error() {
        t.Log("errors agree!")
    } else {
        t.Errorf("Error changed.  Was:  %s, now is: %s", errOrig, errRecall)
    }
}
