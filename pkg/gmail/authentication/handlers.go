package authentication

import (
	"context"
	"encoding/base64"
	"fmt"
	"golang.org/x/net/html"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
)

type Email struct {
	Id      string
	Snippet string
	Raw     string
}

type Emails struct {
	Emails []Email
}

var (
	// Template para a página de autenticação
	templateAuth = `
<html>
	<head>
		<title>Authentication</title>
	</head>
	<body>
		<h1>Authentication</h1>
		<p>Click <a href="https://accounts.google.com/o/oauth2/v2/auth?scope=https://www.googleapis.com/auth/gmail.readonly&access_type=offline&include_granted_scopes=true&state=state_parameter_passthrough_value&redirect_uri=http://localhost:8080/auth&response_type=code&client_id=CLIENT_ID">here</a> to authenticate.</p>
	</body>
</html>
`
	templateAuthenticated = `
<html>
	<head>
		<title>Authenticated</title>
	</head>
	<body>
		<h1>Authenticated</h1>
		<p>Authenticated successfully.</p>

		<div>
			<h2>Unread emails</h2>
			<p>Unread emails:</p>
			<ul>
				{{ range .Emails }}
					<li>
						{{ .Id }} - {{ .Raw }}
					</li>
					
				{{ end }}
			</ul>
		</div>
	</body>
`
	// Configuração do OAuth2
	googleOauthConfig = &oauth2.Config{
		RedirectURL:  "http://localhost:8080/auth",
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		Scopes:       []string{gmail.MailGoogleComScope},
		Endpoint:     google.Endpoint,
	}
	// Variável global para armazenar o token de acesso
	oauthToken *oauth2.Token
)

// Sua função htmlToText
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
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	return text.String()
}

func CallbackHandler(w http.ResponseWriter, r *http.Request) {

	// print all headers and body

	for name, values := range r.Header {
		for _, value := range values {
			fmt.Println(name, value)
		}
	}

	fmt.Println("answer:", r.Body)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(templateAuthenticated)))
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

		emails.Emails = append(emails.Emails, Email{Id: msg.Id, Raw: msg.Raw})
	}

	return emails, nil
}

// extractEmailContent extrai o conteúdo de um e-mail a partir de um objeto *gmail.Message.
// Ele suporta tanto texto plano quanto HTML, convertendo HTML para texto simples.
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

func AuthHandler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if r.URL.Query().Get("code") != "" {

		ctx := context.Background()
		token, err := googleOauthConfig.Exchange(ctx, r.URL.Query().Get("code"))
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal server error"))
			return
		}

		log.Println("Token: ", token.AccessToken)
		config := googleOauthConfig

		log.Println("Config: ", config)
		client := getClient(ctx, config, token)
		gmailService, err := gmail.New(client)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal server error"))
			return
		}
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal server error"))
			return
		}

		log.Println("Gmail Service: ", gmailService)
		emails, err := listUnreadEmails(gmailService, "me")
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal server error"))
			return
		}

		tmpl, err := template.New("authenticated").Parse(templateAuthenticated)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Error parsing template"))
			return
		}

		w.WriteHeader(http.StatusOK)
		tmpl.Execute(w, emails) // Passando os dados dos e-mails para o template
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(templateAuth))
}
