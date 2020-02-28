package model

type PullRequestStats struct {
	Shortest    *BoundaryPullRequest
	Longest     *BoundaryPullRequest
	Durations   []int64
	OpenCount   int32
	ClosedCount int32
}

func (s *PullRequestStats) IncrementOpen() {
	s.OpenCount = s.OpenCount + 1
}

func (s *PullRequestStats) IncrementClosed() {
	s.ClosedCount = s.ClosedCount + 1
}

func (s *PullRequestStats) MergeDurations(durations []int64) {
	if len(durations) > 0 {
		s.Durations = append(s.Durations, durations...)
	}
}

func NewPullRequestStats() *PullRequestStats {
	return &PullRequestStats{
		Shortest:    &BoundaryPullRequest{},
		Longest:     &BoundaryPullRequest{},
		Durations:   make([]int64, 0),
		OpenCount:   0,
		ClosedCount: 0,
	}
}
