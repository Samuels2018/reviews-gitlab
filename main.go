package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"text/template"
	"unicode"

	"github.com/Saluev/kc-review-bot/common"
	"github.com/Saluev/kc-review-bot/hoster/github"
	"github.com/Saluev/kc-review-bot/hoster/gitlab"
	"github.com/Saluev/kc-review-bot/slackermost"
)

func main() {
	var (
		host                 = flag.String("host", "", "host address (e.g. github.com, gitlab.com or self-hosted gitlab url)")
		token                = flag.String("token", "", "host API token")
		repo                 = flag.String("repo", "", "repository (format: 'owner/repo'), or project id (only gitlab)")
		reviewersPath        = flag.String("reviewers", "examples/reviewers.json", "path to the reviewers file")
		templatePath         = flag.String("template", "", "path to the template file")
		webhook              = flag.String("webhook", "", "slack/mattermost webhook URL")
		webhookAuthorization = flag.String("webhook-auth", "", "webhook authorization header")
		channelOrUser        = flag.String("channel", "", "mattermost channel (e.g. MyChannel) or user (e.g. @AnyUser)")
	)
	flag.Parse()

	if *host == "" {
		log.Fatalln("missing host")
	}
	if *repo == "" {
		log.Fatalln("missing repository")
	}

	reviewers := loadReviewers(*reviewersPath)

	var tmpl *template.Template
	if *templatePath != "" {
		tmpl = loadTemplate(*templatePath)
	} else if *host == "github.com" {
		tmpl = github.DefaultTemplate()
	} else {
		tmpl = gitlab.DefaultTemplate()
	}

	var reminder string
	if *host == "github.com" {
		panic("not implemented")
		// 		ownerRespo := strings.SplitN(*repo, "/", 2)
		// 		if len(ownerRespo) != 2 {
		// 			log.Fatalln("wrong repo format (use 'owner/repo')")
		// 		}
		// 		repo, reminders := github.AggregateReminder(*token, ownerRespo[0], ownerRespo[1], reviewers)
		// 		if len(reminders) == 0 {
		// 			// prevent from sending the header only
		// 			return
		// 		}
		// 		reminder = github.ExecTemplate(tmpl, repo, reminders)

	} else {
		project, reminders := gitlab.AggregateReminder(*host, *token, *repo, reviewers)
		if len(reminders) == 0 {
			// prevent from sending the header only
			return
		}
		reminder = gitlab.ExecTemplate(tmpl, project, reminders)
	}

	if reminder == "" {
		return
	}

	fmt.Println(reminder)

	if *webhook != "" {
		if err := slackermost.Send(*channelOrUser, reminder, *webhook, *webhookAuthorization); err != nil {
			log.Fatalf("failed sending slackermost message: %v", err)
		}
	}
}

func loadTemplate(path string) *template.Template {
	t, err := template.ParseFiles(path)
	if err != nil {
		log.Fatalf("failed to read template file: %v", err)
	}
	return t
}

// load reviewers from given json file
// supported formats:
//
//  1. array (preferred):
//     [{"username": "GitLabUserID", "discordId": "<@ChatUserID>", "labels": ["backend"]}, ...]
//
//  2. object with string values (legacy):
//     {"GitLabUserID": "@chatname", ...}
//
//  3. object with email/slack_id values:
//     {"GitLabUserID": {"email": "user@company.com", "slack_id": "U0BN1HW8KQB"}, ...}
//     the slack id is converted to a Slack mention ("<@U...>")
func loadReviewers(path string) map[string]common.Reviewer {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalf("failed to read reviewers file: %v", err)
	}

	reviewers := map[string]common.Reviewer{}

	trimmed := bytes.TrimLeftFunc(b, unicode.IsSpace)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		reviewersFlat := []common.Reviewer{}
		if err := json.Unmarshal(b, &reviewersFlat); err != nil {
			log.Fatalf("failed to unmarshal reviewers: %v", err)
		}
		for _, r := range reviewersFlat {
			reviewers[r.Username] = r
		}
		return reviewers
	}

	raw := map[string]json.RawMessage{}
	if err := json.Unmarshal(b, &raw); err != nil {
		log.Fatalf("failed to unmarshal reviewers: %v", err)
	}

	for username, v := range raw {
		r := common.Reviewer{Username: username}

		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			r.DiscordID = s
			reviewers[username] = r
			continue
		}

		var o struct {
			Email   string `json:"email"`
			SlackID string `json:"slack_id"`
		}
		if err := json.Unmarshal(v, &o); err != nil {
			log.Fatalf("failed to unmarshal reviewer %q: %v", username, err)
		}
		if o.SlackID != "" {
			r.DiscordID = fmt.Sprintf("<@%s>", strings.Trim(o.SlackID, "<@>"))
		} else {
			r.DiscordID = o.Email
		}
		reviewers[username] = r
	}

	return reviewers
}
