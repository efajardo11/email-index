package service

import (
	"sync"
	"sync/atomic"
	"log"
	"github.com/esteban/mail-index/pkg/domain"
)

type EmailWorkerPool struct {
    Paths      chan string
    Emails     chan domain.Email
    ErrorCount int32
    zinc       *ZincClient
}

func NewEmailWorkerPool(zincClient *ZincClient) *EmailWorkerPool {
    return &EmailWorkerPool{
        Paths:  make(chan string, 1000),
        Emails: make(chan domain.Email, 1000),
        zinc:   zincClient,    // This is the only change
    }
}

func (wp *EmailWorkerPool) Start(numWorkers int) {
    var wg sync.WaitGroup
    
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for path := range wp.Paths {
                if email, err := ProcessEmailFile(path); err == nil && email != nil {
                    if err := wp.zinc.IndexEmail(email); err != nil {
                        atomic.AddInt32(&wp.ErrorCount, 1)
                        log.Printf("Error indexing email from %s: %v", path, err)
                        continue
                    }
                    wp.Emails <- *email
                } else if err != nil {
                    atomic.AddInt32(&wp.ErrorCount, 1)
                    log.Printf("Error processing email from %s: %v", path, err)
                }
            }
        }()
    }

    go func() {
        wg.Wait()
        close(wp.Emails)
    }()
}
