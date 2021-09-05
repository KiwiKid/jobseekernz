package gmail

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
)

// Email represents an email fetched from your gmail account.
type Email struct {
	Subject string `json:"subject"`
	Body    string `json:"body"` // Base64.URLEncoding
	Id      string `json:"id"`
	Sender  string `json:"sender"`
}

func (c *GmailClient) Email(id string) (email.Email, error) {
	call := c.srv.Users.Messages.Get(c.email, id)
	res, err := call.Format("full").Do()

	email := email.Email{}

	if err != nil {
		return email, err
	}

	email.Id = id
	email.Body = getMessageBody(res.Payload.Parts)
	email.Sender = getMessageSender(res.Payload.Headers)
	email.Subject = getMessageSubject(res.Payload.Headers)

	return email, nil
}

type GmailClient struct {
	srv   *gmail.Service
	email string
}

func NewGmailClient(email string) GmailClient {
	ctx := context.Background()

	client := getClient(ctx, getConfig())

	srv, err := gmail.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve gmail Client %v", err)
	}

	return GmailClient{srv, email}
}

// getMessageBody finds the HTML body of an email.
func getMessageBody(parts []*gmail.MessagePart) string {
	for _, part := range parts {
		if len(part.Parts) > 0 {
			return getMessageBody(part.Parts)
		} else {
			if part.MimeType == "text/html" {
				return part.Body.Data
			}
		}
	}

	return ""
}

// getMessageSender goes through the headers to find the From header.
func getMessageSender(headers []*gmail.MessagePartHeader) string {
	return getMessageHeader(headers, "From")
}

// getMessageSubject goes through the headers to find the Subject header.
func getMessageSubject(headers []*gmail.MessagePartHeader) string {
	return getMessageHeader(headers, "Subject")
}

// getMessageHeader goes through a list of headers and returns the header where
// the name matches the one we want.
func getMessageHeader(headers []*gmail.MessagePartHeader, wanted string) string {
	for _, header := range headers {
		if header.Name == wanted {
			return header.Value
		}

	}

	return ""
}

// getClient uses a Context and Config to retrieve a Token
// then generate a Client. It returns the generated Client.
func getClient(ctx context.Context, config *oauth2.Config) *http.Client {
	return config.Client(ctx, oauth2Token())
}

// tokenFromFile retrieves a Token from a given file path.
// It returns the retrieved Token and any read error encountered.
func oauth2Token() *oauth2.Token {
	tm, _ := time.Parse("2021-Jan-01", "2026-Jan-01")
	return &oauth2.Token{
		AccessToken:  os.Getenv("GOOGLE_ACCESS_TOKEN"),
		TokenType:    "Bearer",
		RefreshToken: os.Getenv("GOOGLE_REFRESH_TOKEN"),
		Expiry:       tm,
	}
}

func getConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/auth",
			TokenURL: "https://accounts.google.com/o/oauth2/token",
		},
	}
}
