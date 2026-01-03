// Package handlers provides task handlers for the worker.
// Each handler implements the business logic for a specific task type
// and can be registered with the worker to process tasks from the queue.
package handlers

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/nadmax/nexq/internal/task"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

func SendEmailHandler(t *task.Task) error {
	to, ok := t.Payload["to"].(string)
	if !ok {
		return errors.New("missing 'to' field")
	}

	subject, ok := t.Payload["subject"].(string)
	if !ok {
		return errors.New("missing 'subject' field")
	}

	body, ok := t.Payload["body"].(string)
	if !ok {
		return errors.New("missing 'body' field")
	}

	fromName := os.Getenv("FROM_NAME")
	fromAddress := os.Getenv("FROM_ADDRESS")
	from := mail.NewEmail(fromName, fromAddress)
	toEmail := mail.NewEmail("", to)
	email := mail.NewSingleEmail(from, subject, toEmail, body, body)
	client := sendgrid.NewSendClient(os.Getenv("EMAIL_API_KEY"))
	response, err := client.Send(email)
	if err != nil {
		return fmt.Errorf("failed to send emil: %w", err)
	}
	if response.StatusCode >= 400 {
		return fmt.Errorf("sendgrid error: status %d", response.StatusCode)
	}

	log.Printf("Email sent to %s (status: %d)", to, response.StatusCode)
	return nil
}
