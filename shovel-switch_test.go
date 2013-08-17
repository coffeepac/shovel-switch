package main

import (
    ioutil "io/ioutil"
    "testing"
    http "net/http"
    "time"
)

func TestStatusServer(t *testing.T) {
    quitChan = make(chan bool)
    go statusServer(quitChan)

    time.Sleep(1 * time.Millisecond)

    //  test ping
    resp, err := http.Get("http://localhost:7003/ping")
    if err != nil {
        t.Fatalf("Failed to query /ping with %s", err)
    }

    body, err := ioutil.ReadAll(resp.Body)
    if string(body) != "PONG" {
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
