package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/esteban/mail-index/pkg/domain"
	"github.com/esteban/mail-index/pkg/service"
)

func main() {
	numWorkers := flag.Int("workers", runtime.NumCPU(), "Number of workers to process emails")
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to file")
	memprofile := flag.String("memprofile", "", "write memory profile to file")
	flag.Parse()

	// CPU profiling
	if *cpuprofile != "" {
		log.Printf("\n=== PROFILING ===")
		log.Printf("Creating CPU profile at: %s", *cpuprofile)
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		log.Printf("CPU profiling started successfully")
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
		log.Printf("=== PROFILING SETUP COMPLETE ===\n")
	}

	if _, err := os.Stat(domain.EmailsRootFolder); os.IsNotExist(err) {
		log.Fatalf("Email root folder does not exist: %s", domain.EmailsRootFolder)
	}

	// Initialize ZincSearch client
	zinc := service.NewZincClient("http://localhost:4081", "admin", "admin")

	// Create index before starting
	if err := zinc.CreateIndex(); err != nil {
		log.Printf("Warning: Error creating index: %v", err)
		// Continue anyway as index might already exist
	}

	// Create worker pool with ZincSearch client
	wp := service.NewEmailWorkerPool(zinc)
	wp.Start(*numWorkers)

	go func() {
		err := filepath.Walk(domain.EmailsRootFolder, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				log.Printf("Error accessing path %s: %v", path, err)
				return nil
			}
			if !info.IsDir() {
				wp.Paths <- path
			}
			return nil
		})
		if err != nil {
			log.Printf("Error walking directory: %v", err)
		}
		close(wp.Paths)
	}()
	// After the main processing loop, flush any remaining emails
	totalEmails := 0
	startTime := time.Now()
	lastBatchTime := startTime

	for email := range wp.Emails {
		totalEmails++
		if totalEmails%1000 == 0 {
			currentTime := time.Now()
			batchElapsed := currentTime.Sub(lastBatchTime).Seconds()
			totalElapsed := currentTime.Sub(startTime).Seconds()

			stats := zinc.GetStats()
			batchSpeed := 1000.0 / batchElapsed
			totalSpeed := float64(totalEmails) / totalElapsed

			log.Printf("Batch %d complete:\n"+
				"Speed: %.2f emails/sec (avg: %.2f)\n"+
				"Processed: %d, Indexed: %d\n"+
				"Last email: From=%s",
				totalEmails/1000, batchSpeed, totalSpeed,
				stats["total_processed"], stats["total_indexed"],
				email.From)

			lastBatchTime = currentTime
		}
	}

	// Flush remaining emails before summary
	if err := zinc.FlushRemaining(); err != nil {
		log.Printf("Error flushing remaining emails: %v", err)
	}

	elapsed := time.Since(startTime).Seconds()
	emailsPerSecond := float64(totalEmails) / elapsed
	stats := zinc.GetStats()

	log.Printf("\nProcessing Summary:\n"+
		"Total Emails Found: %d\n"+
		"Total Processed: %d\n"+
		"Total Indexed: %d\n"+
		"Total Batches: %d\n"+
		"Processing Speed: %.2f emails/sec\n"+
		"Total Time: %.2f minutes\n",
		totalEmails,
		stats["total_processed"],
		stats["total_indexed"],
		stats["batches_processed"],
		emailsPerSecond,
		elapsed/60)

	// At the end of main, before last error printing:
	// Memory profiling
	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal("could not create memory profile: ", err)
		}
		defer f.Close()
		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatal("could not write memory profile: ", err)
		}
	}
}
