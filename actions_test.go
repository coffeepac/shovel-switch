package main

import (
    ioutil "io/ioutil"
    "time"
    "testing"
)

func TestTBNUp(t *testing.T) {
    flowdcommand = "./echo-params.sh"
    feed := make(chan bool)
    quit := make(chan bool, 1)
    req := make(chan bool,  1)
    resp := make(chan bool,  1)
    go flowdManagement(feed, quit, req, resp, 1, "test", "tbn", true)
    feed <- true
    //  startup and execute once
    time.Sleep(time.Second * 2)

    params, err := ioutil.ReadFile("echoparams")
    if err != nil {
        t.Errorf("Failed to read file echoparams with %s", err)
    }
    if (string(params)) == "up test\n" {
        t.Log("TBN Up successful")
    } else {
        t.Errorf("'up test' expected, got: %s", (string(params)))
    }

    quit <- true
}

func TestTBNDown(t *testing.T) {
    flowdcommand = "./echo-params.sh"
    feed := make(chan bool)
    quit := make(chan bool, 1)
    req := make(chan bool,  1)
    resp := make(chan bool,  1)
    go flowdManagement(feed, quit, req, resp, 1, "test", "tbn", true)
    feed <- false
    //  startup and execute once
    time.Sleep(time.Second * 2)

    params, err := ioutil.ReadFile("echoparams")
    if err != nil {
        t.Errorf("Failed to read file echoparams with %s", err)
    }
    if (string(params)) == "down test\n" {
        t.Log("TBN Up successful")
    } else {
        t.Errorf("'up test' expected, got: %s", (string(params)))
    }

    quit <- true
}

func TestVSATUp(t *testing.T) {
    flowdcommand = "./echo-params.sh"
    feed := make(chan bool)
    quit := make(chan bool, 1)
    req := make(chan bool,  1)
    resp := make(chan bool,  1)
    go flowdManagement(feed, quit, req, resp, 1, "test", "vsat", true)
    feed <- false
    //  startup and execute once
    time.Sleep(time.Second * 2)

    params, err := ioutil.ReadFile("echoparams")
    if err != nil {
        t.Errorf("Failed to read file echoparams with %s", err)
    }
    if (string(params)) == "up test\n" {
        t.Log("TBN Up successful")
    } else {
        t.Errorf("'up test' expected, got: %s", (string(params)))
    }

    quit <- true
}
