package gitlab

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xanzy/go-gitlab"

	"github.com/Saluev/kc-review-bot/common"
)

func TestAggregateReminder(t *testing.T) {
	mockedClient := &clientWrapperMock{
		loadProjectFunc: func(repo interface{}) gitlab.Project {
			return gitlab.Project{Name: "mocked project"}
		},
		loadMRsFunc: func(repo interface{}) []*gitlab.MergeRequest {
			return []*gitlab.MergeRequest{
				{Title: "MR0", Labels: []string{"backend"}},
				{Title: "MR1", WorkInProgress: true},
			}
		},
		loadApprovalsFunc: func(repo interface{}, mr *gitlab.MergeRequest) *gitlab.MergeRequestApprovals {
			return &gitlab.MergeRequestApprovals{
				Approvers: []*gitlab.MergeRequestApproverUser{
					{User: &gitlab.BasicUser{Username: "groot"}},
				},
			}
		},
		loadEmojisFunc: func(repo interface{}, mr *gitlab.MergeRequest) []*gitlab.AwardEmoji {
			return []*gitlab.AwardEmoji{
				{Name: thumbsup, User: struct {
					Name      string `json:"name"`
					Username  string `json:"username"`
					ID        int    `json:"id"`
					State     string `json:"state"`
					AvatarURL string `json:"avatar_url"`
					WebURL    string `json:"web_url"`
				}{Username: "random"}},
			}
		},
		loadDiscussionsFunc: func(repo interface{}, mr *gitlab.MergeRequest) []*gitlab.Discussion {
			return []*gitlab.Discussion{
				{ID: "id0", Notes: []*gitlab.Note{{Resolved: false, Resolvable: true}}},
			}
		},
	}

	reviewers := map[string]common.Reviewer{
		"spidy": {Username: "spidy", DiscordID: "<@spidy>"},
		"groot": {Username: "groot", DiscordID: "@groot", Labels: []string{"backend"}},
	}

	expP := gitlab.Project{
		Name: "mocked project",
	}

	expR := []reminder{
		{MR: &gitlab.MergeRequest{Title: "MR0", Labels: []string{"backend"}}, Missing: []string{"<@spidy>"}, Emojis: map[string]int{"thumbsup": 1}, Discussions: 1},
	}

	gotP, gotR := aggregate(mockedClient, 2009901, reviewers)

	require.Equal(t, expP, gotP)
	require.Equal(t, expR, gotR)
}

func TestResponsiblePerson(t *testing.T) {
	t.Run("author", func(t *testing.T) {
		mr := &gitlab.MergeRequest{Author: &gitlab.BasicUser{Name: "name-of-author"}}
		reviewers := map[string]common.Reviewer{}
		got := responsiblePerson(mr, reviewers)
		require.Equal(t, "name-of-author", got)
	})

	t.Run("@author", func(t *testing.T) {
		mr := &gitlab.MergeRequest{Author: &gitlab.BasicUser{Username: "gitlab_name"}}
		reviewers := map[string]common.Reviewer{"gitlab_name": {Username: "gitlab_name", DiscordID: "@author-of-mr"}}
		got := responsiblePerson(mr, reviewers)
		require.Equal(t, "@author-of-mr", got)
	})

	t.Run("assignee", func(t *testing.T) {
		mr := &gitlab.MergeRequest{Assignee: &gitlab.BasicUser{Username: "gitlab_name"}}
		reviewers := map[string]common.Reviewer{"gitlab_name": {Username: "gitlab_name", DiscordID: "assignee-of-mr"}}
		got := responsiblePerson(mr, reviewers)
		require.Equal(t, "assignee-of-mr", got)
	})
}

func TestGetReviewed(t *testing.T) {
	mr := &gitlab.MergeRequest{Author: &gitlab.BasicUser{Username: "mr_author"}}

	approvals := &gitlab.MergeRequestApprovals{
		Approvers: []*gitlab.MergeRequestApproverUser{
			{User: &gitlab.BasicUser{Username: "approver0"}},
		},
	}

	type user struct {
		Name      string `json:"name"`
		Username  string `json:"username"`
		ID        int    `json:"id"`
		State     string `json:"state"`
		AvatarURL string `json:"avatar_url"`
		WebURL    string `json:"web_url"`
	}

	emojis := []*gitlab.AwardEmoji{
		{Name: thumbsup, User: user{Username: "user0"}},
		{Name: thumbsdown, User: user{Username: "user1"}},
		{Name: sleeping, User: user{Username: "user2"}},
		{Name: "hooray", User: user{Username: "user3"}},
		{Name: thumbsup, User: user{Username: "user3"}},
		{Name: "anyemoji", User: user{Username: "user4"}},
	}

	got := getReviewed(mr, approvals, emojis)

	want := []string{"mr_author", "approver0", "user0", "user1", "user2", "user3"}
	require.Equal(t, want, got)
}

func TestMissingReviewers(t *testing.T) {
	reviewedBy := []string{"user1", "user2"}

	mr := &gitlab.MergeRequest{Labels: []string{"backend"}}

	approvers := map[string]common.Reviewer{
		"user0": {Username: "user0", DiscordID: "@user0"},
		"user1": {Username: "user1", DiscordID: "@user1"},
		"user2": {Username: "user2", DiscordID: "@user2", Labels: []string{"backend"}},
		"user3": {Username: "user3", DiscordID: "@user3", Labels: []string{"frontend"}},
	}

	got := missingReviewers(mr, reviewedBy, approvers)

	want := []string{"@user0"}
	require.ElementsMatch(t, want, got)
}

func TestMissingReviewersEmptyLabels(t *testing.T) {
	// reviewers without labels are relevant for every merge request
	reviewedBy := []string{}

	mr := &gitlab.MergeRequest{Labels: []string{"anything"}}

	approvers := map[string]common.Reviewer{
		"user0": {Username: "user0", DiscordID: "@user0"},
		"user1": {Username: "user1", DiscordID: "@user1"},
	}

	got := missingReviewers(mr, reviewedBy, approvers)

	want := []string{"@user0", "@user1"}
	require.ElementsMatch(t, want, got)
}

func TestAggregateEmojis(t *testing.T) {
	input := []*gitlab.AwardEmoji{
		{Name: "emoji0"},
		{Name: "emoji0"},
		{Name: "emoji0"},
		{Name: "emoji1"},
		{Name: "emoji1"},
		{Name: "emoji2"},
	}

	got := aggregateEmojis(input)

	want := map[string]int{"emoji0": 3, "emoji1": 2, "emoji2": 1}
	require.Equal(t, want, got)
}
