package main

import (
	"flag"
	"io/ioutil"
	"time"
	"strings"
	"math/rand"
	"sync"
	"os"
	"os/signal"
	"syscall"
	"strconv"
	"sync/atomic"
	"github.com/sirupsen/logrus"
	"log"
	"bytes"
	"context"
	"net/http"
	"net/url"
	"encoding/json"
	"github.com/satori/go.uuid"
)

//type Worker struct {
//	Address  string `json:"address"`
//	Label    string `json:"label"`
//	Capacity int    `json:"capacity"`
//}

type RequestSim struct {
	RequestId	string	`json:"request_id"`
	ProcId		string	`json:"proc_id"`
}

var (
	destUrl = &url.URL{
		Scheme:		"http",
	}
	destAddr = flag.String("dest_addr", "10.199.1.227:3001", "")
	pathList = flag.String("path_list", "", "")
	requestPrefix = "cccf"
	httpClient = &http.Client{
		Transport:		&http.Transport{
			MaxIdleConns:			0,
			MaxIdleConnsPerHost:	100,
		}, }
	concurrency int64 = 200
	sendAmount int64 = 0
	failureNum int64 = 0
	failureLimit int64 = 1000
	stopping = make(chan bool)
	totalRespTime float64 = 0
	tt = transactionTime{
		lt:			0,
		st:			time.Hour,
	}
	rtt = time.Second * 10
)

type transactionTime struct {
	lt		time.Duration
	st		time.Duration
	rt		time.Duration
	sync.Mutex
}

type Message struct {
	Msg		string		`json:"message"`
}

func waitSignal(cf context.CancelFunc) {
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	s := <-sigChan
	logrus.Errorf("*****receive signal[%v]******", s)
	cf()
}

func main() {
	if *destAddr == "" {
		log.Fatalf("No destAddr specified.\n")
	}
	if *pathList == "" {
		log.Fatalf("No path specified.\n")
	}
	destUrl.Host = *destAddr
	paths := strings.Split(*pathList, ",")
	ctx, cf := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}
	startPoint := time.Now()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	f := func(ctx context.Context, path string) {
		wg.Add(1)
		Url := *destUrl
		Url.Path = path
		reqMsg := &RequestSim{
			RequestId:		requestPrefix + strconv.Itoa(r.Intn(1000000)),
			ProcId:			uuid.NewV4().String(),
		}
		raw, _ := json.Marshal(reqMsg)
		var longT time.Duration
		var shortT time.Duration
		var respT time.Duration
		var totalSend  int64
//		responseRecord := make(map[string]int)

		longT = 0
		shortT = time.Hour
		respT = 0
//		log.Printf("raw: %v", raw)
mainloop: for {
			select {
			case <-ctx.Done():
				break mainloop
			case <-time.After(0):
			}
			req, _:= http.NewRequest("POST", Url.String(), bytes.NewReader(raw))
			req.Header.Set("Content-Type", "application/json")
//			req.Header.Set("Keep-Alive", "300")
//			log.Infof("send req out to %s, ProcId:%v, path:%v", Url.String(), reqMsg.ProcId, path)
//			atomic.AddInt32(&sendAmount, 1)

			st := time.Now()
			resp, err := httpClient.Do(req)
			last := time.Now().Sub(st)

			totalSend++
			if err != nil {
				log.Printf("[Error] time:%v(s) proc_id:%v, err:%v", last.Seconds(), reqMsg.ProcId, err)
				totalFail := atomic.AddInt64(&failureNum, 1)
				if totalFail > failureLimit {
					cf()
					break
				}
				continue
			}
//			log.Infof("receive resp: %v", resp)
			if last > longT {
				longT = last
			}
			if last < shortT {
				shortT = last
			}
			respT += last
			respBody, _ := ioutil.ReadAll(resp.Body)
			defer resp.Body.Close()

			Msg := &Message{}
			json.Unmarshal(respBody, Msg)
//			log.Infof("Receive resp for path(%s):%d, msg: %v", path, resp.StatusCode, Msg)
			log.Printf("%v time:%v(s) proc_id:%v, msg:%v", resp.StatusCode, last.Seconds(), reqMsg.ProcId, Msg)
//			time.Sleep(rtt)
		}
		tt.Lock()
		defer tt.Unlock()
		if longT > tt.lt {
			tt.lt = longT
		}
		if shortT < tt.st {
			tt.st = shortT
		}
		tt.rt += respT
		sendAmount += totalSend
		wg.Done()
	}

	for i:=0; i<int(concurrency); i++ {
		go f(ctx, paths[i % len(paths)])
	}
	go waitSignal(cf)
	wg.Wait()
	runedTime := time.Now().Sub(startPoint)

	log.Printf("%-*s %*d hits", 30, "Transactions:", 10, sendAmount)
	log.Printf("%-*s %*.3f %%", 30, "Availability:", 10, float64((sendAmount-failureNum)*100/sendAmount))
	log.Printf("%-*s %*.3f secs", 30, "Elapsed time:", 10, runedTime.Seconds())
	log.Printf("%-*s %*.3f secs", 30, "Average Responce time:", 10, tt.rt.Seconds()/float64(sendAmount-failureNum))
	log.Printf("%-*s %*.3f t/s", 30, "Transaction rate:", 10, float64(sendAmount)/runedTime.Seconds())
	log.Printf("%-*s %*.3f ", 30, "Concurrency:", 10, (float64(sendAmount)/runedTime.Seconds())/float64(2*concurrency))
//	log.Printf("%-*s %*.3f ", 30, "Concurrency:", 10, float64(sendAmount)/runedTime.Seconds())
	log.Printf("%-*s %*d ", 30, "Successful Transactions:", 10, sendAmount-failureNum)
	log.Printf("%-*s %*d ", 30, "Failed Transactions:", 10, failureNum)
	log.Printf("%-*s %*.3f ", 30, "Longest Transactions:", 10, tt.lt.Seconds())
	log.Printf("%-*s %*.3f ", 30, "Shortest Transactions:", 10, tt.st.Seconds())
}
