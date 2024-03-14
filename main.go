package main

import (
	"context"
	"encoding/base64"
	"golang.org/x/net/html"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"log"
	"net/http"
	"os"
	"strings"
)

var (
	Token string

	googleOauthConfig = &oauth2.Config{
		RedirectURL:  "http://localhost:8080/auth",
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		Scopes:       []string{gmail.MailGoogleComScope},
		Endpoint:     google.Endpoint,
	}
)

type Email struct {
	Id      string
	Snippet string
	Raw     string
	Body    string // Adicionado para armazenar o conteúdo formatado do email
}

type Emails struct {
	Emails []Email
}

func htmlToText(htmlContent string) string {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		// lidar com erro
		return ""
	}
	var text strings.Builder
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.TextNode {
			text.WriteString(n.Data)
		} else if n.Type == html.ElementNode && n.Data == "p" {
			text.WriteString("\n\n") // Adiciona quebra de linha após cada parágrafo
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	return text.String()
}

func extractEmailContent(msg *gmail.Message) (string, error) {
	if msg.Payload == nil {
		return "", nil // Sem conteúdo para processar
	}

	// Verifica se o MIME Type é text/plain ou text/html
	switch msg.Payload.MimeType {
	case "text/plain":
		data, err := base64.URLEncoding.DecodeString(msg.Payload.Body.Data)
		if err != nil {
			return "", err
		}
		return string(data), nil

	case "text/html":
		data, err := base64.URLEncoding.DecodeString(msg.Payload.Body.Data)
		if err != nil {
			return "", err
		}
		return htmlToText(string(data)), nil
	}

	// Se o MIME Type não é text/plain ou text/html, procura nos parts
	for _, part := range msg.Payload.Parts {
		switch part.MimeType {
		case "text/plain", "text/html":
			data, err := base64.URLEncoding.DecodeString(part.Body.Data)
			if err != nil {
				return "", err
			}
			if part.MimeType == "text/html" {
				return htmlToText(string(data)), nil
			}
			return string(data), nil
		}
	}

	return "", nil // Sem conteúdo reconhecível
}

func getMessageContent(service *gmail.Service, userEmail, messageId string) (Email, error) {
	var email Email
	msg, err := service.Users.Messages.Get(userEmail, messageId).Format("full").Do()
	if err != nil {
		return email, err
	}

	email.Id = msg.Id
	emailContent, err := extractEmailContent(msg)
	if err != nil {
		return email, err
	}

	email.Raw = emailContent
	return email, nil
}

func getClient(ctx context.Context, config *oauth2.Config, token *oauth2.Token) *http.Client {
	return config.Client(ctx, token)
}

func listUnreadEmails(service *gmail.Service, userEmail string) (Emails, error) {
	var emails Emails

	r, err := service.Users.Messages.List(userEmail).Q("is:unread").MaxResults(5).Do()
	if err != nil {
		log.Printf("Error listing unread emails: %v", err)
		return emails, err
	}

	for _, l := range r.Messages {
		msg, err := getMessageContent(service, userEmail, l.Id)
		if err != nil {
			log.Printf("Error getting message content: %v", err)
			continue
		}

		// Convertendo o conteúdo RAW para texto simples e armazenando em msg.Body
		msg.Body = htmlToText(msg.Raw)

		// Preencher o campo Snippet com os primeiros 100 caracteres do corpo do email
		if len(msg.Body) > 100 {
			msg.Snippet = msg.Body[:100] + "..."
		} else {
			msg.Snippet = msg.Body
		}

		emails.Emails = append(emails.Emails, msg)
	}

	return emails, nil
}

func ReadEmails() {

	var ctx = context.Background()
	var oauthToken *oauth2.Token
	var token *oauth2.Token

	jwtFile := "token.json"
	tokenFromFile, err := os.ReadFile(jwtFile)
	if err != nil {
		log.Println("Error while reading token file: ", err)
		tokenFromFile = nil
	} else {
		oauthToken = &oauth2.Token{
			AccessToken: string(tokenFromFile),
		}
	}

	token = oauthToken

	config := googleOauthConfig

	client := getClient(ctx, config, token)
	gmailService, err := gmail.New(client)
	if err != nil {
		log.Println(err)
		return
	}
	if err != nil {
		log.Println(err)
		return
	}
	emails, err := listUnreadEmails(gmailService, "me")
	if err != nil {
		log.Println(err)
		return
	}

	log.Println("Emails: ", emails)

}

func init() {
	token, err := os.ReadFile("token.json")
	if err != nil {
		log.Println("Error while reading token file: ", err)
		return
	}
	contentFile := string(token)
	Token = contentFile
}

func main() {

	log.Println("Start GoMail")

	ReadEmails()
}
