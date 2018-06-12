/*
   Copyright 2018 Marc Fouché

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package main

import (
	"crypto/md5"
	"encoding/hex"
	"flag"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sync"
	"syscall"

	"github.com/go-redis/redis"
	"github.com/golang/glog"
	"github.com/kelseyhightower/envconfig"
)

// Configuration holds all the configuration for gohtmltopdf
type Configuration struct {
	WK    WKConfiguration
	Redis RedisConfiguration
}

// WKConfiguration holds the options for wkhtmltopdf
type WKConfiguration struct {
	Path      string `default:"wkhtmltopdf"`                                                               // path to the wkhtmltopdf binary
	Options   string `default:"--quiet --enable-external-links --print-media-type --javascript-delay 300"` // Options passed to wkhtmltopdf
	Directory string `default:"/var/go-teemoproject"`                                                      // directory where the pdfs are written
}

// RedisConfiguration holds all the configuration for redis
type RedisConfiguration struct {
	Address      string `default:"localhost:6379"`  // address to redis
	Queue        string `default:"go-teemoproject"` // queue where the jobs are sent
	Password     string `default:""`                // password of the redis server
	DB           int    `default:"0"`               // db used
	WriteToRedis bool   `default:"false"`
}

// Job represents a job
type Job struct {
	JobSum string
	URL    string
}

// NewJob returns a Job
func NewJob(url string) Job {
	jobSum := GetMD5Hash(url)
	return Job{jobSum, url}
}

// GetJobs retrieves jobs from redis
func GetJobs(redisClient *redis.Client, queue string, jobs chan<- Job, done <-chan bool) {
	for {
		val, err := redisClient.BLPop(0, queue).Result()
		if err != nil {
			glog.Errorf("ERROR: cannot get job from Redis: %v", err)
			continue
		}
		job := NewJob(val[1])
		select {
		case <-done:
			glog.Infof("Closing jobs channel, sending back message: %v", val[1])
			if err := redisClient.LPush(queue, val[1]).Err(); err != nil {
				glog.Errorf("ERROR: message lost ?: %v: %v", job, err)
			}
			return
		case jobs <- job:
		}
	}
}

// HTMLToPdfWorker get jobs from the jobs chan and executes the wkhtmlcommand
func HTMLToPdfWorker(id int, wg *sync.WaitGroup, WKconf WKConfiguration, jobs <-chan Job) {
	glog.V(1).Infof("Woker %d: starting", id)
	for {
		select {
		case job, more := <-jobs:
			if more {
				glog.Infof("Worker %d: started job: %v", id, job)
				cmdLine := WKconf.Path + " " + WKconf.Options + " " + job.URL + " " + WKconf.Directory + "/" + job.JobSum + ".pdf"
				glog.V(1).Infof("Worker %d: executing command: %s", id, cmdLine)
				cmd := exec.Command("bash", "-c", cmdLine)
				// Give the subprocess another Pgid so it doesn't catch the
				// kill signals directeo to go-teemoproject
				cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}
				output, err := cmd.CombinedOutput()
				if err != nil {
					glog.Errorf("worker %d: ERROR: %v: %v: %s", id, job, err, output)
				} else {
					glog.Infof("Worker %d: finished job: %v", id, job)
				}
			} else {
				glog.Infof("Worker %d: done", id)
				defer wg.Done()
				return
			}
		}
	}
}

// GetMD5Hash returns the md5sum of a string
func GetMD5Hash(text string) string {
	hasher := md5.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}

func main() {
	flag.Parse()
	// Read conf from environment variables
	var conf Configuration
	err := envconfig.Process("redis", &conf.Redis)
	if err != nil {
		glog.Error(err)
		os.Exit(1)
	}
	err = envconfig.Process("wkhtmltopdf", &conf.WK)
	if err != nil {
		glog.Error(err)
		os.Exit(1)
	}
	glog.Infof("%+v", conf)

	// Create redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     conf.Redis.Address,
		Password: conf.Redis.Password,
		DB:       conf.Redis.DB,
	})

	// Get the number of logical CPUs seen. This will be the max number of
	// workers
	numCPU := runtime.NumCPU()
	glog.Infof("Number of CPU seen: %d", numCPU)

	jobs := make(chan Job, 1)

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go GetJobs(redisClient, conf.Redis.Queue, jobs, done)

	var wg sync.WaitGroup
	for i := 0; i < numCPU; i++ {
		wg.Add(1)
		go HTMLToPdfWorker(i, &wg, conf.WK, jobs)
	}

	sig := <-sigs
	done <- true
	glog.Info(sig)
	glog.Info("Exiting")
	glog.Info("Waiting for all jobs to be done")
	close(jobs)
	wg.Wait()
	glog.Info("All jobs done, bye !")
}
