package jenkins

import (
    "time"
    "fmt"
    "testing"
    http "net/http"
)

var (
    dummyJenkinsLatch = false
)

func dummyJenkins(){
    if dummyJenkinsLatch {
        return
    } else {
        dummyJenkinsLatch = true
        http.HandleFunc("/promote", promoteHandle)

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

func TestStartGood(t *testing.T) {
    go dummyJenkins()

    //  let server start
    time.Sleep(time.Second)

    ciURL = "http://localhost:7005/promote"
    local := &PromoteToShip{Shipcode: "local"}
    err := local.Start()
    if err != nil {
        t.Errorf("Failed start command with: %s", err)
    } else {
        t.Log("First postform a success!")
    }
}
