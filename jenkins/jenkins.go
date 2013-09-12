package jenkins

import (
  "strconv"
  "errors"
//  json "encoding/json"
  http "net/http"
  httputil "net/http/httputil"
  url  "net/url"
)

var (
  ciURL = "http://jenkins-cd.mtnsatcloud.com/job/promote-to-ship/buildWithParameters"
)

/*
**  Holds data needed to promote a job from central chef to ship chef
*/
type PromoteToShip struct {
    Shipcode    string
}

/*
**  Start - queues up the promote to ship job with this Shipcode on jenkins-ci
*/
func (p *PromoteToShip) Start() (err error) {
    //  post the job!
    resp, err := http.PostForm(ciURL,url.Values{"SHIPNAME": {p.Shipcode}})
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
*/
func (p *PromoteToShip) Wait() {
}
