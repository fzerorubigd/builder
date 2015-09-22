package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"sync"
)

var (
	script   = flag.String("script", "", "the script to run")
	route    = flag.String("route", "", "the script to run")
	webhook  = flag.String("slack", "", "the slack webhook url")
	channel  = flag.String("channel", "", "slack channel")
	username = flag.String("user", "", "slack user name")
	bind     = flag.String("bind", ":9415", "bind on address:port")
)

// Payload is the gitlab hook payload
type Payload struct {
	After   string `json:"after"`
	Before  string `json:"before"`
	Commits []struct {
		Author struct {
			Email string `json:"email"`
			Name  string `json:"name"`
		} `json:"author"`
		ID        string `json:"id"`
		Message   string `json:"message"`
		Timestamp string `json:"timestamp"`
		URL       string `json:"url"`
	} `json:"commits"`
	ObjectKind string `json:"object_kind"`
	ProjectID  int    `json:"project_id"`
	Ref        string `json:"ref"`
	Repository struct {
		Description     string `json:"description"`
		GitHTTPURL      string `json:"git_http_url"`
		GitSSHURL       string `json:"git_ssh_url"`
		Homepage        string `json:"homepage"`
		Name            string `json:"name"`
		URL             string `json:"url"`
		VisibilityLevel int    `json:"visibility_level"`
	} `json:"repository"`
	TotalCommitsCount int    `json:"total_commits_count"`
	UserEmail         string `json:"user_email"`
	UserID            int    `json:"user_id"`
	UserName          string `json:"user_name"`
}

// SlackPayload the slack payload
type SlackPayload struct {
	Channel     string            `json:"channel"`
	Text        string            `json:"text"`
	Username    string            `json:"username"`
	IconURL     string            `json:"icon_url,omitempty"`
	IconEmoji   string            `json:"icon_emoji,omitempty"`
	Parse       string            `json:"parse"`
	Attachments []SlackAttachment `json:"attachments"`
}

// SlackAttachment the attachment
type SlackAttachment struct {
	Color   string `json:"color"`
	Text    string `json:"text"`
	PreText string `json:"pretext,omitempty"`
	Title   string `json:"title,omitempty"`
}

// SlackDoMessage Try to send message to configured slack channel
func SlackDoMessage(err interface{}, icon string, attacments ...SlackAttachment) {
	payload := &SlackPayload{}
	payload.Channel = *channel

	title := fmt.Errorf("cannot extract title, the type is %T, value is %v", err, err)
	switch err.(type) {
	case string:
		title = fmt.Errorf(err.(string))
	case error:
		title = err.(error)
	}

	payload.Text = title.Error()
	payload.Username = *username
	payload.Parse = "full" // WTF?
	if icon != "" {
		if icon[0] == ':' {
			payload.IconEmoji = icon
		} else {
			payload.IconURL = icon
		}
	}

	payload.Attachments = attacments

	encoded, err := json.Marshal(payload)
	if err != nil {
		log.Print(err)
		return
	}

	resp, err := http.PostForm(*webhook, url.Values{"payload": {string(encoded)}})
	if err != nil {
		log.Print(err)
		return
	}

	if resp.StatusCode != http.StatusOK {
		log.Print("sending payload to slack failed")
		return
	}
}

func main() {
	flag.Parse()
	if *script == "" || *route == "" || *bind == "" {
		log.Fatal("must pass all argument. see the help")
	}

	lock := sync.Mutex{}
	http.HandleFunc(
		"/"+*route,
		func(w http.ResponseWriter, r *http.Request) {
			pl := Payload{}
			decoder := json.NewDecoder(r.Body)
			err := decoder.Decode(&pl)

			var gitlab bool
			if err == nil {
				gitlab = true
			}
			go func() {
				lock.Lock()
				defer lock.Unlock()

				cmd := exec.Command("/bin/bash", "-x", *script)
				stdout, err := cmd.CombinedOutput()

				if err != nil {
					if gitlab {
						if *username != "" {
							go SlackDoMessage(
								err,
								":shit:",
								SlackAttachment{Text: pl.UserName + "<" + pl.UserEmail + ">", Color: "#AA3939"},
								SlackAttachment{Text: string(stdout), Color: "#AA3939"},
							)
						}
					} else {
						go SlackDoMessage(
							err,
							":shit:",
							SlackAttachment{Text: "Direct url call", Color: "#FFEEFF"},
							SlackAttachment{Text: string(stdout)},
						)
					}
					return
				}

				if *username != "" {
					if gitlab {
						go SlackDoMessage(
							"build was ok",
							":+1:",
							SlackAttachment{Text: pl.UserName + "<" + pl.UserEmail + ">", Color: "#FFEEFF"},
							SlackAttachment{Text: string(stdout)},
						)
					} else {
						SlackDoMessage(
							"build was ok",
							":+1:",
							SlackAttachment{Text: "Direct url call", Color: "#FFEEFF"},
							SlackAttachment{Text: string(stdout)},
						)
					}
				}
			}()
		})
	log.Fatal(http.ListenAndServe(*bind, nil))
}
