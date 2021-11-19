package main

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/osbuild/osbuild-composer/internal/jobqueue/dbjobqueue"
)

func main() {
	logrus.SetReportCaller(true)

	archs := []string{"x86_64"}
	jobType := "osbuild"
	// 14 days
	cutoff := time.Now().Add(-(time.Hour * 24 * 14))

	var conf Config
	LoadConfigFromEnv(&conf)
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		conf.PGUser,
		conf.PGPassword,
		conf.PGHost,
		conf.PGPort,
		conf.PGDatabase,
		conf.PGSSLMode,
	)
	jobs, err := dbjobqueue.New(dbURL)
	if err != nil {
		panic(err)
	}

	maxCReqs, err := strconv.Atoi(conf.MaxConccurentRequests)
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		logrus.Info("Cleaning up AWS")
		// TODO multiple regions?
		err := AWSCleanup(maxCReqs, conf.AWSAccessKeyID, conf.AWSSecretAccessKey, "us-east-1", cutoff)
		if err != nil {
			logrus.Errorf("AWS cleanup failed: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		logrus.Info("Cleaning up GCP")
		credentials, err := ioutil.ReadFile(conf.GCPCredentialsFile)
		if err != nil {
			logrus.Errorf("Unable to read GCP credentials: %v", err)
		}
		err = GCPCleanup(maxCReqs, credentials, cutoff)
		if err != nil {
			logrus.Errorf("GCP Cleanup failed: %v", err)
		}
	}()

	wg.Wait()
	logrus.Info("🦀🦀🦀 cloud cleanup done 🦀🦀🦀")

	var jobTypes []string
	for _, a := range archs {
		jobTypes = append(jobTypes, fmt.Sprintf("%s:%s", jobType, a))
	}
	jobsByType, err := jobs.JobsUptoByType(jobTypes, cutoff)
	if err != nil {
		logrus.Errorf("Error getting jobIds: %v", err)
		return
	}
	for k, v := range jobsByType {
		logrus.Infof("Deleting jobs and their dependencies of type %v", k)
		for _, jobId := range v {
			err = jobs.DeleteJobIncludingDependencies(jobId)
			if err != nil {
				logrus.Errorf("Error deleting job: %v", jobId)
			}
		}
	}
	logrus.Info("🦀🦀🦀 dbqueue cleanup done 🦀🦀🦀")
}
