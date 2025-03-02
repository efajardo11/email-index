package domain

import (
	"io"
	"net/mail"
	"os"
)

type Email struct {
	MessageID string `json:"message_id"`
	Date      string `json:"date"`
	From      string `json:"from"`
	To        string `json:"to"`
	Subject   string `json:"subject"`
	Content   string `json:"content"`
	Filepath  string `json:"filepath"`
}

const EmailsRootFolder = "database/maildir"

func (e *Email) Parse() error {
	file, err := os.Open(e.Filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	msg, err := mail.ReadMessage(file)
	if err != nil {
		return err
	}

	e.MessageID = msg.Header.Get("Message-ID")
	e.Date = msg.Header.Get("Date")
	e.From = msg.Header.Get("From")
	e.To = msg.Header.Get("To")
	e.Subject = msg.Header.Get("Subject")

	// Read the message body
	body, err := io.ReadAll(msg.Body)
	if err != nil {
		return err
	}
	e.Content = string(body)

	return nil
}
