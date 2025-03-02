package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "log"
    "net/http"

    "github.com/esteban/mail-index/pkg/domain"
    "github.com/esteban/mail-index/pkg/service"
)

func testZincUpload(email *domain.Email) error {
    jsonData, err := json.Marshal(email)
    if err != nil {
        return fmt.Errorf("error creating JSON: %v", err)
    }

    url := "http://localhost:4080/api/enron/_doc"
    req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
    if err != nil {
        return fmt.Errorf("error creating request: %v", err)
    }

    req.SetBasicAuth("admin", "admin")
    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return fmt.Errorf("error sending request: %v", err)
    }
    defer resp.Body.Close()

    fmt.Printf("ZincSearch Response Status: %s\n", resp.Status)
    return nil
}

// Change main to a named function
func testEmailProcessing() {
    testFile := `C:\Users\Esteban\Documents\mail-index\database\maildir\allen-p\_sent_mail\1`

    email, err := service.ProcessEmailFile(testFile)
    if err != nil {
        log.Fatalf("Error processing email: %v", err)
    }

    fmt.Printf("Original file: %s\n", testFile)
    fmt.Printf("Parsed Date: %s\n", email.Date)

    jsonData, err := json.MarshalIndent(email, "", "    ")
    if err != nil {
        log.Fatalf("Error creating JSON: %v", err)
    }

    fmt.Println("\nJSON output:")
    fmt.Println(string(jsonData))

    // Test uploading to ZincSearch
    if err := testZincUpload(email); err != nil {
        log.Printf("ZincSearch upload failed: %v", err)
    }
}

// New main that calls both tests
func main() {
    fmt.Println("Testing email processing...")
    testEmailProcessing()
}
