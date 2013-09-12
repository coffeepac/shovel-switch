package jenkins

import (
  "time"
  "strconv"
  "errors"
  json "encoding/json"
  http "net/http"
  httputil "net/http/httputil"
  url  "net/url"
)

var (
  ciURL     = "http://jenkins-cd.mtnsatcloud.com/job/promote-to-ship/"
  ciPostURL = ciURL + "buildWithParameters"
  ciLastBuild = ciURL + "api/json?depth=1&tree=lastBuild[actions[parameters[name,value]]],url"
  ciResult  = "api/json?tree=result"
)

/*
**  Holds data needed to promote a job from central chef to ship chef
*/
type PromoteToShip struct {
    Shipcode    string
    started bool
    waited  bool
    olderr  error
}

/*
**  struct representation of the lastBuild json rest response
*/
type lastBuildResponse struct {
    LastBuild struct {
        Actions []struct {
            Parameters []struct {
                Name    string  `json:"name"`
                Value   string  `json:"value"`
            }   `json:"parameters"`
        }   `json:"actions"`
        Url string  `json:"url"`
    }   `json:"lastBuild"`
}

/*
**  struct rep for result resp response
*/
type jobResult struct {
    Result  interface{}  `json:"result"`  //  null is a valid bit here, thats not a string
}

/*
**  Start - queues up the promote to ship job with this Shipcode on jenkins-ci
*/
func (p *PromoteToShip) Start() (err error) {
    p.started = true
    p.waited = false  //  is default, but if we call posting twice against same jenkins reference
    //  post the job!
    resp, err := http.PostForm(ciPostURL,url.Values{"SHIPNAME": {p.Shipcode}})
    if err != nil {
        return err
    } else if resp.StatusCode != 201 {
        dumpResponse, _ := httputil.DumpResponse(resp, true)
        return errors.New("Should have gotten a 201 from the job post, received a " + strconv.Itoa(resp.StatusCode) + " with a body of: " + string(dumpResponse))
    }
    return nil
}

/*
**  Wait - polls the jenkins-ci server until the job started above:
**         + starts and has an job number
**           - does this by polling the last built job and checking
**             the job parameter
**         + has a result
**           - result is a field in the json status
**           - if there is a result, the job is done
**
**  TODO:  add a timeout so we don't loop forever
*/
func (p *PromoteToShip) Wait(sleepSeconds int) (err error) {
    defer func() {
        p.olderr = err
    }()

    //  only let this go if we have started and haven't waited
    if !p.started {
        return errors.New("Must call Start before waiting for the job to finish")
    } else if p.waited {
        return p.olderr  //  not sure if we want to do something to indicate this error has been returned already
    }
    //  poll lastBuild until our job shows up there.
    //  lastBuild shows current build, if there is one
    jobNotStarted := true
    var lastBuild lastBuildResponse
    for jobNotStarted {
        resp, err := http.Get(ciLastBuild)
        if err != nil {
            return err
        }

        decoder := json.NewDecoder(resp.Body)
        err = decoder.Decode(&lastBuild)
        resp.Body.Close()
        if err != nil {
            return err
        }

        if lastBuild.LastBuild.Actions[0].Parameters[0].Value == p.Shipcode {
            jobNotStarted = false
        }
        time.Sleep(time.Duration(sleepSeconds) * time.Second)
    }

    //  have the job, poll the job url until a result is present
    jobRunning := true
    var result jobResult
    for jobRunning {
        resp, err := http.Get(lastBuild.LastBuild.Url + ciResult)
        if err != nil {
            return err
        }

        decoder := json.NewDecoder(resp.Body)
        err = decoder.Decode(&result)
        resp.Body.Close()
        if err != nil {
            return err
        }

        if result.Result != nil {
            jobRunning = false
        }

        time.Sleep(time.Duration(sleepSeconds) * time.Second)
    }

    if result.Result.(string) != "SUCCESS" {
        err = errors.New("Job did not succeed.  Result is: " + result.Result.(string))
    } else {
        err = nil
    }

    return err
}
