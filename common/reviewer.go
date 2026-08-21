package common

type Reviewer struct {
	Username  string   `json:"username"`
	DiscordID string   `json:"discordId"`
	Labels    []string `json:"labels"`
}
