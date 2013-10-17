package main

import (
    "fmt"
    "log"
    "encoding/json"
    "encoding/base64"
    "code.google.com/p/go.net/websocket"
    "github.com/garyburd/redigo/redis"
)

var server string = ":6379"
var bufsize int = 1024
var batchsize int = 128
var wsbatch int = 64
var queuename string = "metrics"
var ctlname string = "metrics-control"


type RedisReader struct {
    rpool *redis.Pool
}

type WebsocketConnPool struct {
    size int
    reader chan websocket.Conn
}

func (p *WebsocketConnPool) Initialize(size int, config *websocket.Config) error {
    p.size = size
    return nil;
}


type Tags struct {
    Device string `json:"device"`
}

type Metric struct {
    Timestamp float64 `json:"timestamp"`
    Metric string `json:"metric"`
    Value float64 `json:"value"`
    Tags Tags `json:"tags"`
}

type Packet struct {
    Control string `json:"control"`
    Metrics []Metric `json:"metrics"`
}

func dial () (redis.Conn, error) {
     c, err := redis.Dial("tcp", server)
     if err != nil {
         return nil, err
     }
     _, err = c.Do("SELECT", "0")
     if err != nil {
         return nil, err
     }
     return c, nil
}

func drain (conn redis.Conn, ch chan Metric) {
    for {
        var rangeresult []string
        var ltrim int

        conn.Send("MULTI")
        conn.Send("LRANGE", queuename, 0, batchsize-1)
        conn.Send("LTRIM", queuename, batchsize, -1)
        r, err := redis.Values(conn.Do("EXEC"))
        if err != nil {
            // handle error
        }
        if _, err := redis.Scan(r, &rangeresult, &ltrim); err != nil {
            // handle error
        }
        if len(rangeresult) == 0 {
            break;
        }
        for _, dp := range rangeresult {
            var m Metric
            json.Unmarshal([]byte(dp), &m)
            ch <- m
        }
    }
}

func subscribe () (inbox chan Metric, err error) {
    inbox = make(chan Metric, bufsize)
    conn, _ := dial()
    conn2, _ := dial()
    psc := redis.PubSubConn{Conn: conn2}
    go func () {
        for {
            fmt.Println("Subscribing")
            psc.Subscribe(ctlname)
            for {
                m := psc.Receive()
                switch m.(type) {
                    case redis.Message:
                        fmt.Println("Waking up to read")
                        psc.Unsubscribe()
                        drain(conn, inbox)
                        break
                    case redis.Subscription:
                        continue
                    case error:
                        return
                }
            }
            fmt.Println("Time to sleep")
        }
    }()
    return inbox, nil;
}

func forward(outbox chan Metric, cfg websocket.Config) {
    ws, err := websocket.DialConfig(&cfg)
    if err != nil {
        log.Fatal(err)
    }
    for {
        metrics := make([]Metric, 64)
        for i:=0; i<wsbatch; i++ {
            metrics = append(metrics, <-outbox)
            if err != nil {
                log.Fatal(err)
            }
        }
        p := map[string]interface{}{
            "control": nil,
            "metrics": metrics,
        }
        websocket.JSON.Send(ws, p)
    }
}

func main() {
    inbox , _ := subscribe()
    origin := "http://localhost/"
    url := "ws://localhost:8080/ws/metrics/store"
    config, err := websocket.NewConfig(url, origin)
    if err != nil {
        return;
    }
    data := []byte("admin:zenoss")
    str := base64.StdEncoding.EncodeToString(data)
    config.Header.Add("Authorization", "basic " + str)
    forward(inbox, *config)
}
