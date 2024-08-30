package main

import (
	"encoding/base64"
	"log"
	"os"
	"time"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

type Email struct {
	Name    string
	Address string
}

func getClient() *sendgrid.Client {
	apiKey := os.Getenv("SENDGRID_API_KEY")
	if apiKey == "" {
		log.Fatal("SENDGRID_API_KEY is not set")
		return nil
	}

	return sendgrid.NewSendClient(apiKey)
}

func SetupEmail() {
	apiKey := os.Getenv("SENDGRID_API_KEY")
	if apiKey == "" {
		log.Fatal("SENDGRID_API_KEY is not set")
	}
}

func SendEmail(tos []*mail.Email, file string) bool {
	fileContent, err := os.ReadFile(file)
	if err != nil {
		log.Printf("error: %v", err)
		return false
	}

	client := getClient()
	if client == nil {
		return false
	}

	m := mail.NewV3Mail()
	m.SetFrom(mail.NewEmail("Scan", "scanner@example.com"))
	m.Subject = "Scan"
	c := mail.NewContent("text/plain", "scanned file")
	m.AddContent(c)

	// c = mail.NewContent("text/html", "s")
	// m.AddContent(c)

	p := mail.NewPersonalization()
	p.AddTos(tos...)
	m.AddPersonalizations(p)

	a := mail.NewAttachment()
	encoded := base64.StdEncoding.EncodeToString(fileContent)
	a.SetContent(encoded)
	a.SetType("application/pdf")
	a.SetFilename(time.Now().Format("2006.01.02-15.04") + "-scan.pdf")
	a.SetDisposition("attachment")
	m.AddAttachment(a)

	log.Println("sending email")
	response, err := client.Send(m)
	if err != nil {
		log.Println(err)
		return false
	} else {
		if response.StatusCode != 202 && response.StatusCode != 200 {
			log.Println(response.StatusCode)
			log.Println(response.Body)
			log.Println(response.Headers)
			return false
		}
	}
	log.Println("email sent")
	return true
}
