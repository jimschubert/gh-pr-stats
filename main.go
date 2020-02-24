package main

import (
	"context"
	"fmt"
	"github.com/google/go-github/v29/github"
	"github.com/hako/durafmt"
	"github.com/montanaflynn/stats"
	"os"
	"time"
)
import "golang.org/x/oauth2"
import "github.com/jessevdk/go-flags"

var opts struct {
	Owner string `short:"o" long:"owner" description:"GitHub Owner/Org name"`

	Repo string `short:"r" long:"repo" description:"GitHub Repo name"`

	Start string `short:"s" long:"start" description:"Start date in format YYYY-mm-dd"`

	End string `short:"e" long:"end" description:"End date in format YYYY-mm-dd"`

	Enterprise string `long:"enterprise" description:"GitHub Enterprise URL in the format http(s)://[hostname]/api/v3"`

	Verbose bool `long:"verbose" description:"Display verbose messages"`
}

type boundaryPullRequest struct {
	PullRequest *github.PullRequest
	Duration    int64
}

func (b *boundaryPullRequest) Update(pr *github.PullRequest, duration int64) {
	b.PullRequest = pr
	b.Duration = duration
}

type pullRequestStats struct {
	shortest    *boundaryPullRequest
	longest     *boundaryPullRequest
	durations   []int64
	openCount   int32
	closedCount int32
}

func (s *pullRequestStats) IncrementOpen() {
	s.openCount = s.openCount + 1
}

func (s *pullRequestStats) IncrementClosed() {
	s.closedCount = s.closedCount + 1
}

func (s *pullRequestStats) MergeDurations(durations []int64) {
	if len(durations) > 0 {
		s.durations = append(s.durations, durations...)
	}
}

func NewPullRequestStats() *pullRequestStats {
	return &pullRequestStats{
		shortest:    &boundaryPullRequest{},
		longest:     &boundaryPullRequest{},
		durations:   make([]int64, 0),
		openCount:   0,
		closedCount: 0,
	}
}

var start time.Time
var end time.Time

func retrievePullRequests(client *github.Client, pullOpts *github.PullRequestListOptions, prStats *pullRequestStats) (int, error) {
	pulls, _, err := client.PullRequests.List(context.Background(), opts.Owner, opts.Repo, pullOpts)
	if err != nil {
		return 0, err
	}

	durations := make([]int64, 0)
	for _, pull := range pulls {
		if start.Unix() < pull.CreatedAt.Unix() && end.Unix() > pull.CreatedAt.Unix() {
			var f int64
			if pull.ClosedAt == nil {
				f = time.Now().Unix() - pull.CreatedAt.Unix()
			} else {
				f = pull.ClosedAt.Unix() - pull.CreatedAt.Unix()
			}

			if *pull.State == "open" {
				prStats.IncrementOpen()
			} else {
				prStats.IncrementClosed()
			}

			durations = append(durations, f)

			if prStats.shortest.PullRequest == nil || f < prStats.shortest.Duration {
				prStats.shortest.Update(pull, f)
			}
			if prStats.longest.PullRequest == nil || prStats.longest.Duration < f {
				prStats.longest.Update(pull, f)
			}

			if opts.Verbose {
				fmt.Printf("Pull (%s): %s closed after %d seconds\n", *pull.State, *pull.Title, f)
			}

			prStats.MergeDurations(durations)
		} else {

			prStats.MergeDurations(durations)
			// we've hit a limit, stop processing
			return 0, nil
		}
	}
	return len(pulls), nil
}

func main() {
	args, err := flags.Parse(&opts)
	if err != nil {
		flagError := err.(*flags.Error)
		if flagError.Type == flags.ErrHelp {
			return
		}
		if flagError.Type == flags.ErrUnknownFlag {
			fmt.Println("Unknown flag. Please use --help for available options.")
			return
		}
		fmt.Printf("Error parsing command line options: %s\n", err)
		return
	}
	// only accept switches, no args
	if len(args) > 0 {
		fmt.Printf("Unknown command line argument '%s'.\n", args[0])
		return
	}

	start, err = time.Parse("2006-01-02", opts.Start)
	if err != nil {
		start = time.Unix(0, 0)
	}

	end, err = time.Parse("2006-01-02", opts.End)
	if err != nil {
		end = time.Now()
	}

	ctx := context.Background()
	token, found := os.LookupEnv("GITHUB_TOKEN")
	if !found {
		fmt.Println("Environment variable GITHUB_TOKEN not found.")
		os.Exit(1)
	}
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	var client *github.Client
	if len(opts.Enterprise) > 0 {
		client, err = github.NewEnterpriseClient(opts.Enterprise, opts.Enterprise, tc)
	} else {
		client = github.NewClient(tc)
	}

	pullOpts := &github.PullRequestListOptions{
		State:       "all",
		Base:        "master",
		Sort:        "created",
		Direction:   "desc",
		ListOptions: github.ListOptions{Page: 1, PerPage: 50},
	}

	prStats := NewPullRequestStats()

	for {
		count, err := retrievePullRequests(client, pullOpts, prStats)

		if err != nil {
			fmt.Printf("Error retrieving pull requests '%s.\n'", err)
			os.Exit(1)
		}

		if count <= 0 {
			break
		}

		pullOpts.Page = pullOpts.Page + 1
	}

	data := stats.LoadRawData(prStats.durations)
	minimum, _ := stats.Min(data)
	median, _ := stats.Median(data)
	maximum, _ := stats.Max(data)

	fmt.Printf("Pull Requests:\n\tOpen: %d\n\tClosed: %d\n", prStats.openCount, prStats.closedCount)
	fmt.Printf("Open Duration:\n")
	fmt.Printf("\tMinimum: %s\n", durafmt.Parse(time.Duration(int64(minimum))*time.Second))
	fmt.Printf("\tMedian: %s\n", durafmt.Parse(time.Duration(int64(median))*time.Second))
	fmt.Printf("\tMaximum: %s\n", durafmt.Parse(time.Duration(int64(maximum))*time.Second))

	if prStats.shortest.PullRequest != nil {
		fmt.Printf("Shortest-lived PR:\n\tTitle: %s\n\tURL: %s\n\tAuthor: %s\n", *prStats.shortest.PullRequest.Title, *prStats.shortest.PullRequest.HTMLURL, prStats.shortest.PullRequest.GetUser().GetLogin())
	}
	if prStats.longest.PullRequest != nil {
		fmt.Printf("Longest-lived PR:\n\tTitle: %s\n\tURL: %s\n\tAuthor: %s\n", *prStats.longest.PullRequest.Title, *prStats.longest.PullRequest.HTMLURL, prStats.shortest.PullRequest.GetUser().GetLogin())
	}
}
