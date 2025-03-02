package service

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/esteban/mail-index/pkg/domain"
)

const (
	dateFormat       = "2006-01-02T15:04:05Z"
	maxContentLength = 50000 // Ajusta este valor según sea necesario
)

func ProcessEmailFile(filepathString string) (*domain.Email, error) {
	if strings.HasSuffix(filepathString, ".") {
		return nil, nil
	}

	file, err := os.ReadFile(filepath.Clean(filepathString))
	if err != nil {
		return nil, err
	}

	content := string(file)
	headerEnd := -1
	for _, sep := range []string{"\r\n\r\n", "\n\n"} {
		if idx := strings.Index(content, sep); idx != -1 {
			headerEnd = idx
			break
		}
	}

	if headerEnd == -1 {
		return nil, fmt.Errorf("invalid email format in file: %s", filepathString)
	}

	headerSection := content[:headerEnd]
	bodySection := strings.TrimLeft(content[headerEnd:], "\r\n")

	// Truncate content if it's too long
	if len(bodySection) > maxContentLength {
		bodySection = bodySection[:maxContentLength] + "..."
	}

	email := &domain.Email{
		Filepath: filepathString,
		Content:  bodySection,
	}

	var currentHeader string
	var currentValue string

	for _, line := range strings.Split(headerSection, "\n") {
		line = strings.TrimRight(line, "\r")

		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if currentValue != "" {
				currentValue += " "
			}
			currentValue += strings.TrimSpace(line)
			continue
		}

		if currentHeader != "" {
			switch currentHeader {
			case "Message-ID":
				email.MessageID = strings.Trim(currentValue, "<>")
			case "Date":
				parsedDate, err := parseEmailDate(currentValue)
				if err != nil {
					log.Printf("Warning: Could not parse date %s in file %s: %v", currentValue, filepathString, err)
					email.Date = currentValue
				} else {
					email.Date = parsedDate
				}
			case "From":
				email.From = extractEmailAddress(currentValue)
			case "To":
				email.To = extractEmailAddress(currentValue)
			case "Subject":
				email.Subject = currentValue
			}
		}

		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			currentHeader = strings.TrimSpace(parts[0])
			currentValue = strings.TrimSpace(parts[1])
		}
	}

	if email.MessageID == "" || email.Date == "" || email.From == "" {
		return nil, fmt.Errorf("missing required headers in file: %s", filepathString)
	}

	return email, nil
}

func parseEmailDate(dateStr string) (string, error) {
	formats := []string{
		"Mon, 2 Jan 2006 15:04:05 -0700 (PST)",
		"Mon, 2 Jan 2006 15:04:05 -0700 (PDT)",
		"Mon, 2 Jan 2006 15:04:05 -0700 (MST)",
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST",
		"2 Jan 2006 15:04:05 -0700",
		time.RFC1123Z,
		time.RFC822Z,
		time.RFC850,
		time.ANSIC,
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t.UTC().Format(dateFormat), nil
		}
	}
	return "", fmt.Errorf("unable to parse date: %s", dateStr)
}

func extractEmailAddress(value string) string {
	if start := strings.LastIndex(value, "<"); start != -1 {
		if end := strings.LastIndex(value, ">"); end != -1 && end > start {
			return value[start+1 : end]
		}
	}
	return strings.TrimSpace(value)
}
