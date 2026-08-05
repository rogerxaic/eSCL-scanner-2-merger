package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/resend/resend-go/v2"
)

func SetupEmail() {
	if os.Getenv("RESEND_API_KEY") == "" {
		log.Print("warning: RESEND_API_KEY is not set, email sending disabled")
	}
}

func SendEmail(recipients []string, file string) bool {
	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		log.Print("RESEND_API_KEY is not set, cannot send email")
		return false
	}

	fileContent, err := os.ReadFile(file)
	if err != nil {
		log.Printf("error: %v", err)
		return false
	}

	client := resend.NewClient(apiKey)

	now := time.Now()
	fileName := now.Local().Format("2006.01.02-15.04") + "-scan.pdf"

	from := os.Getenv("EMAIL_FROM")
	if from == "" {
		from = "Scan <scanner@example.com>"
	}

	log.Println("sending email")
	sent, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    from,
		To:      recipients,
		Subject: "Scan",
		Text: fmt.Sprintf(
			"Scanned file on %s. Please find the attached file '%s'.",
			now.Local().Format("2 Jan 2006 at 15:04"),
			fileName,
		),
		Attachments: []*resend.Attachment{{
			Content:     fileContent,
			Filename:    fileName,
			ContentType: "application/pdf",
		}},
	})
	if err != nil {
		log.Printf("error sending email: %v", err)
		return false
	}
	log.Printf("email sent: %s", sent.Id)
	return true
}
