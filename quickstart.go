package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

// Contains tells whether a contains x.
func FindByName(labels []*gmail.Label, x string) (*gmail.Label, error) {
	for _, n := range labels {
		if x == n.Name {
			return n, nil
		}
	}
	return nil, errors.New("label not found")
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

// utils:
func (box *LookupSet) add(item Lookup) *LookupSet {
	box.Lookups = append(box.Lookups, item)
	return box
}

type LookupResult struct {
	Data       string
	Label      string
	LookupUsed *Lookup
}

type LookupSet struct {
	Lookups       []Lookup
	LookupResults []LookupResult
	GmailLabel    string
}

type Lookup struct {
	DataRegex  string
	LabelRegex string
}

func main() {

	ctx := context.Background()
	b, err := ioutil.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, gmail.GmailReadonlyScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Gmail client: %v", err)
	}

	user := "me"
	l, err := srv.Users.Labels.List(user).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve labels: %v", err)
	}
	if len(l.Labels) == 0 {
		fmt.Println("No labels found.")
		return
	}
	//	fmt.Println("Labels:")
	//	for _, l := range l.Labels {
	//		fmt.Printf("- %s %s\n", l.Name, l.Id)
	//	}

	// labels

	seekLabel, err := FindByName(l.Labels, "Seek")
	if err != nil {
		log.Fatalf("Unable to find label matching: seek %v", err)
	}

	e, err := srv.Users.Messages.List(user).LabelIds(seekLabel.Id).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve messages: %v", err)
	}
	if len(e.Messages) == 0 {
		fmt.Println("No Emails found.")
		return
	}

	// TODO: only get todays email
	allContent := make([]*gmail.Message, len(e.Messages))
	for i, mess := range e.Messages {
		c, err := srv.Users.Messages.Get(user, mess.Id).Do()

		if err != nil {
			log.Fatalf("Unable to retrieve email content : %v", err)
		}

		allContent[i] = c
	}

	var lookupResults = []*LookupResult{}

	fmt.Println("Emails content:")
	for _, content := range allContent {
		body := getMessageBody(content.Payload.Parts)
		//fmt.Printf("======================")
		// Replace "-" and "_" to allow normal decoding
		// https://stackoverflow.com/questions/24812139/base64-decoding-of-mime-email-not-working-gmail-api
		decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(strings.ReplaceAll(body, "-", "+"), "_", "/"))
		if err != nil {
			fmt.Println("decode error:", err)
			return
		}

		allLookups := &LookupSet{
			GmailLabel: "Seek",
			Lookups: []Lookup{
				{
					DataRegex:  `ve found <b>\s*(\d+)\s*</b>`,
					LabelRegex: `<b>software in (.+)</b> posted a short while`,
				},
			},
		}

		// MAYBEDO: Don't parse the whole html here, just the section we need.
		// (pro - quicker parsing? con - makes find data logic more complex)
		for _, lookup := range allLookups.Lookups {

			// Find number:
			dataRegex := regexp.MustCompile(lookup.DataRegex)
			dataMatch := dataRegex.FindStringSubmatch(string(decoded))

			// Find number:
			labelRegex := regexp.MustCompile(lookup.LabelRegex)
			labelMatch := labelRegex.FindStringSubmatch(string(decoded))

			if dataMatch != nil && labelMatch != nil {

				res := &LookupResult{
					Data:  dataMatch[1],
					Label: labelMatch[1],
				}

				lookupResults = append(lookupResults, res)
			} else {
				fmt.Printf("No Data (%+v) or Label (%+v)\n", dataMatch, labelMatch)
			}
		}

	}
	for _, l := range lookupResults {
		fmt.Printf("%s: %s\n", l.Label, l.Data)
	}

}
