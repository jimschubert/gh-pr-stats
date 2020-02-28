package model

import (
	"github.com/google/go-github/v29/github"
	"time"
)

type PullRequestDetails struct {
	ID           int64   `csv:"id"`
	Title        string  `csv:"title"`
	Author       string  `csv:"author"`
	MergedBy     string  `csv:"merged_by"`
	CreatedAt    string  `csv:"created_at"`
	UpdatedAt    string  `csv:"updated_at"`
	ClosedAt     *string `csv:"closed_at,omitempty"`
	MergedAt     *string `csv:"merged_at,omitempty"`
	Draft        bool    `csv:"draft"`
	Merged       bool    `csv:"merged"`
	Locked       bool    `csv:"locked"`
	Commits      int     `csv:"commits"`
	Comments     int     `csv:"comments"`
	Additions    int     `csv:"additions"`
	Deletions    int     `csv:"deletions"`
	ChangedFiles int     `csv:"changed_files"`
	Url          string  `csv:"url"`
}

func (p *PullRequestDetails) FromGitHubPullRequest(pr *github.PullRequest) {
	if p == nil {
		return
	}

	if pr == nil {
		return
	}

	p.ID = pr.GetID()
	p.Locked = pr.GetLocked()
	p.Title = pr.GetTitle()
	p.Author = pr.GetUser().GetLogin()
	if mb := pr.GetMergedBy(); mb != nil {
		p.MergedBy = mb.GetLogin()
	} else {
		p.MergedBy = ""
	}
	p.Url = pr.GetHTMLURL()
	p.CreatedAt = pr.GetCreatedAt().Format(time.RFC3339)
	if pr.ClosedAt != nil {
		if closedAt := pr.GetClosedAt().Format(time.RFC3339); closedAt != "" {
			p.ClosedAt = &closedAt
		}
	}
	if pr.MergedAt != nil {
		if mergedAt := pr.GetMergedAt().Format(time.RFC3339); mergedAt != "" {
			p.MergedAt = &mergedAt
		}
	}
	p.Draft = pr.GetDraft()
	p.Merged = pr.GetMerged()
	p.Commits = pr.GetCommits()
	p.Comments = pr.GetComments()
	p.Additions = pr.GetAdditions()
	p.Deletions = pr.GetDeletions()
}
