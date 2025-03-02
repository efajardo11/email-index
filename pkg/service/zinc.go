package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/esteban/mail-index/pkg/domain"
)

type ZincClient struct {
	baseURL   string
	username  string
	password  string
	client    *http.Client
	batch     []*domain.Email
	batchSize int
	mu        sync.Mutex
	stats     struct {
		totalProcessed   int64
		totalIndexed     int64
		batchesProcessed int64
		errors           map[string]int64
	}
}

func NewZincClient(baseURL, username, password string) *ZincClient {
    return &ZincClient{
        baseURL:   baseURL,
        username:  username,
        password:  password,
        client:    &http.Client{},
        batchSize: 1000,
        stats: struct {
            totalProcessed   int64
            totalIndexed     int64
            batchesProcessed int64
            errors          map[string]int64
        }{
            errors: make(map[string]int64),
        },
    }
}

func (z *ZincClient) IndexEmail(email *domain.Email) error {
	z.mu.Lock()
	z.batch = append(z.batch, email)
	currentBatchSize := len(z.batch)
	z.mu.Unlock()

	if currentBatchSize >= z.batchSize {
		return z.flushBatch()
	}
	return nil
}

func (z *ZincClient) flushBatch() error {
	z.mu.Lock()
	if len(z.batch) == 0 {
		z.mu.Unlock()
		return nil
	}
	currentBatch := z.batch
	z.batch = make([]*domain.Email, 0, z.batchSize)
	z.mu.Unlock()

	atomic.AddInt64(&z.stats.batchesProcessed, 1)
	atomic.AddInt64(&z.stats.totalProcessed, int64(len(currentBatch)))

	var bulkBuilder strings.Builder
	for _, email := range currentBatch {
		// Add index action line
		indexLine := fmt.Sprintf(`{"index": {"_index": "enron"}}%s`, "\n")
		bulkBuilder.WriteString(indexLine)

		// Add document line
		emailJSON, err := json.Marshal(email)
		if err != nil {
			return fmt.Errorf("error marshaling email: %v", err)
		}
		bulkBuilder.Write(emailJSON)
		bulkBuilder.WriteString("\n")
	}

	url := fmt.Sprintf("%s/api/_bulk", z.baseURL)
	req, err := http.NewRequest("POST", url, strings.NewReader(bulkBuilder.String()))
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	req.SetBasicAuth(z.username, z.password)
	req.Header.Set("Content-Type", "application/x-ndjson")

	resp, err := z.client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		var zincError struct {
		    Error string `json:"error"`
		    Code  int    `json:"code"`
		}
		if err := json.Unmarshal(bodyBytes, &zincError); err != nil {
		    // If can't parse JSON, use raw response
		    errMsg := fmt.Sprintf("status_%d", resp.StatusCode)
		    z.mu.Lock()
		    z.stats.errors[errMsg]++
		    z.mu.Unlock()
		    return fmt.Errorf("zinc error: %s - %s", resp.Status, string(bodyBytes))
		}
		
		// Use structured error for stats
		errMsg := fmt.Sprintf("status_%d_%s", zincError.Code, zincError.Error)
		z.mu.Lock()
		z.stats.errors[errMsg]++
		z.mu.Unlock()
		return fmt.Errorf("zinc error: %s - %s", resp.Status, zincError.Error)
	}

	atomic.AddInt64(&z.stats.totalIndexed, int64(len(currentBatch)))
	return nil
}

// Add method to get statistics
func (z *ZincClient) GetStats() map[string]int64 {
	z.mu.Lock()
	defer z.mu.Unlock()

	stats := map[string]int64{
		"total_processed":   z.stats.totalProcessed,
		"total_indexed":     z.stats.totalIndexed,
		"batches_processed": z.stats.batchesProcessed,
	}

	for errType, count := range z.stats.errors {
		stats["error_"+errType] = count
	}

	return stats
}

// Add this method to create index
func (z *ZincClient) CreateIndex() error {
	mapping := map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"message_id": map[string]string{"type": "keyword"},
				"date":       map[string]string{"type": "date"},
				"from":       map[string]string{"type": "keyword"},
				"to":         map[string]string{"type": "text"},
				"subject":    map[string]string{"type": "text"},
				"content":    map[string]string{"type": "text"},
				"filepath":   map[string]string{"type": "keyword"},
			},
		},
	}

	jsonData, err := json.Marshal(mapping)
	if err != nil {
		return fmt.Errorf("error marshaling index mapping: %v", err)
	}

	url := fmt.Sprintf("%s/api/index", z.baseURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	req.SetBasicAuth(z.username, z.password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := z.client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("error creating index: %s - %s", resp.Status, string(bodyBytes))
	}

	return nil
}

// Add this method to flush remaining emails
func (z *ZincClient) FlushRemaining() error {
	return z.flushBatch()
}
