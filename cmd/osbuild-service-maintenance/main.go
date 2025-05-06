package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

func main() {
	// 14 days
	cutoff := time.Now().Add(-(time.Hour * 24 * 14))
	log.Printf("Cutoff date: %v", cutoff)

	conf := Config{
		MaxConcurrentRequests: 20,
		EnableDBMaintenance:   false,
		EnableGCPMaintenance:  false,
		EnableAWSMaintenance:  false,
	}
	err := LoadConfigFromEnv(&conf)
	if err != nil {
		log.Fatal(err)
	}

	if conf.DryRun {
		log.Println("Dry run, no state will be changed")
	}

	if conf.MaxConcurrentRequests == 0 {
		log.Fatal("Max concurrent requests is 0")
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if !conf.EnableAWSMaintenance {
			log.Println("AWS maintenance not enabled, skipping")
			return
		}

		log.Println("Cleaning up AWS")
		err := AWSCleanup(conf.MaxConcurrentRequests, conf.DryRun, conf.AWSAccessKeyID, conf.AWSSecretAccessKey, cutoff)
		if err != nil {
			log.Printf("AWS cleanup failed: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if !conf.EnableGCPMaintenance {
			log.Println("GCP maintenance not enabled, skipping")
			return
		}

		log.Println("Cleaning up GCP")
		var gcpConf GCPCredentialsConfig
		err := LoadConfigFromEnv(&gcpConf)
		if err != nil {
			log.Println("Unable to load GCP config from environment")
			return
		}

		if !gcpConf.valid() {
			log.Println("GCP credentials invalid, fields missing")
			return
		}

		creds, err := json.Marshal(&gcpConf)
		if err != nil {
			log.Printf("Unable to marshal gcp conf: %v", err)
			return
		}

		err = GCPCleanup(creds, conf.MaxConcurrentRequests, conf.DryRun, cutoff)
		if err != nil {
			log.Printf("GCP Cleanup failed: %v", err)
		}
	}()

	wg.Wait()
	log.Println("🦀🦀🦀 cloud cleanup done 🦀🦀🦀")

	if !conf.EnableDBMaintenance {
		log.Println("🦀🦀🦀 DB maintenance not enabled, skipping  🦀🦀🦀")
		return
	}
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		conf.PGUser,
		conf.PGPassword,
		conf.PGHost,
		conf.PGPort,
		conf.PGDatabase,
		conf.PGSSLMode,
	)
	err = DBCleanup(dbURL, conf.DryRun, cutoff)
	if err != nil {
		log.Fatalf("Error during DBCleanup: %v", err)
	}
	log.Println("🦀🦀🦀 dbqueue cleanup done 🦀🦀🦀")
}
