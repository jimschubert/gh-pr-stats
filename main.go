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

	Verbose bool `long:"verbose" description:"Display verbose messages"`
}

type boundaryPullRequest struct {
	PullRequest *github.PullRequest
	Duration int64
}

func (b *boundaryPullRequest) Update(pr *github.PullRequest, duration int64) {
	b.PullRequest = pr
	b.Duration = duration
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
	client := github.NewClient(tc)
	
	pullOpts := &github.PullRequestListOptions{
		State:       "all",
		Base:        "master",
		Sort:        "created",
		Direction:   "desc",
		ListOptions: github.ListOptions{Page: 1, PerPage: 50},
	}

	var shortest = &boundaryPullRequest{}
	var longest = &boundaryPullRequest{}

	// how many seconds PR has been open
	var durations = make([]int64, 0)

	for {
		d, count, err := retrievePullRequests(client, pullOpts, shortest, longest, durations)
		durations = d

		if err != nil {
			fmt.Printf("Error retrieving pull requests '%s.\n'", err)
			os.Exit(1)
		}

		if count <= 0 {
			break
		}

		pullOpts.Page = pullOpts.Page + 1
	}

	data := stats.LoadRawData(durations)
	minimum, _ := stats.Min(data)
	median, _ := stats.Median(data)
	maximum, _ := stats.Max(data)

	fmt.Printf("Minimum: %s\n", durafmt.Parse(time.Duration(int64(minimum))*time.Second))
	fmt.Printf("Median: %s\n", durafmt.Parse(time.Duration(int64(median))*time.Second))
	fmt.Printf("Maximum: %s\n", durafmt.Parse(time.Duration(int64(maximum))*time.Second))

	if shortest.PullRequest != nil {
		fmt.Printf("Shortest PR\n\tTitle: %s\n\tURL: %s\n", *shortest.PullRequest.Title, *shortest.PullRequest.URL)
	}
	if longest.PullRequest != nil {
		fmt.Printf("Longest PR\n\tTitle: %s\n\tURL: %s\n", *longest.PullRequest.Title, *longest.PullRequest.URL)
	}
}

func retrievePullRequests(client *github.Client, pullOpts *github.PullRequestListOptions, shortest *boundaryPullRequest, longest *boundaryPullRequest, durations []int64) ([]int64, int, error) {
	pulls, _, err := client.PullRequests.List(context.Background(), opts.Owner, opts.Repo, pullOpts)
	if err != nil {
		return durations, 0, err
	}

	start, err := time.Parse("2006-01-02", opts.Start)
	if err != nil {
		start = time.Unix(0, 0)
	}

	end, err := time.Parse("2006-01-02", opts.End)
	if err != nil {
		end = time.Now()
	}

	for _, pull := range pulls {
		if start.Unix() < pull.CreatedAt.Unix() && end.Unix() > pull.CreatedAt.Unix() {
			var f int64
			if pull.ClosedAt == nil {
				f = time.Now().Unix() - pull.CreatedAt.Unix()
			} else {
				f = pull.ClosedAt.Unix() - pull.CreatedAt.Unix()
			}

			durations = append(durations, f)

			if shortest.PullRequest == nil || f < shortest.Duration {
				shortest.Update(pull, f)
			}
			if longest.PullRequest == nil || longest.Duration < f {
				longest.Update(pull, f)
			}

			if opts.Verbose {
				fmt.Printf("Pull (%s): %s closed after %d seconds\n", *pull.State, *pull.Title, f)
			}
		} else {
			// we've hit a limit, stop processing
			return durations, 0, nil
		}
	}
	return durations, len(pulls), nil
}
