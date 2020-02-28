package model

import "github.com/google/go-github/v29/github"

type BoundaryPullRequest struct {
	PullRequest *github.PullRequest
	Duration    int64
}

func (b *BoundaryPullRequest) Update(pr *github.PullRequest, duration int64) {
	b.PullRequest = pr
	b.Duration = duration
}