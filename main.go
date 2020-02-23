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

	// TODO: List PRs
	//opt := &github.RepositoryListOptions{Type: "all", Affiliation: "owner", ListOptions: github.ListOptions{PerPage: 300}}
	//fmt.Println(opts.Owner)
	//repos, _, err := client.Repositories.List(ctx, opts.Owner, opt)
	//for _, repo := range repos {
	//	fmt.Println(*repo.Name)
	//}

	pullOpts := &github.PullRequestListOptions{
		State:       "all",
		Base:        "master",
		Sort:        "created",
		Direction:   "desc",
		ListOptions: github.ListOptions{Page: 1, PerPage: 20},
	}
	pulls, _, err := client.PullRequests.List(context.Background(), opts.Owner, opts.Repo, pullOpts)
	if err != nil {
		fmt.Printf("Error retrieving pull requests '%s.\n'", err)
		os.Exit(1)
	}

	start, err := time.Parse("2006-01-02", opts.Start)
	if err != nil {
		start = time.Unix(0, 0)
	}

	end, err := time.Parse("2006-01-02", opts.End)
	if err != nil {
		end = time.Now()
	}

	// how many seconds PR has been open
	durations := make([]int64, 0)

	var shortest *boundaryPullRequest
	var longest *boundaryPullRequest
	for _, pull := range pulls {
		if start.Unix() < pull.CreatedAt.Unix() && end.Unix() > pull.CreatedAt.Unix() {
			var f int64
			if pull.ClosedAt == nil {
				f = time.Now().Unix() - pull.CreatedAt.Unix()
			} else {
				f = pull.ClosedAt.Unix() - pull.CreatedAt.Unix()
			}

			durations = append(durations, f)

			if shortest == nil || shortest.Duration > f {
				shortest = &boundaryPullRequest{
					PullRequest: pull,
					Duration:    f,
				}
			}
			if longest == nil || longest.Duration < f {
				longest = &boundaryPullRequest{
					PullRequest: pull,
					Duration:    f,
				}
			}

			if opts.Verbose {
				fmt.Printf("Pull (%s): %s closed after %f seconds\n", *pull.State, *pull.Title, f)
			}
		}
	}

	data := stats.LoadRawData(durations)
	median, _ := stats.Median(data)
	minimum, _ := stats.Min(data)
	maximum, _ := stats.Max(data)
	fmt.Printf("Median: %s\n", durafmt.Parse(time.Duration(int64(median))*time.Second))
	fmt.Printf("Minimum: %s\n", durafmt.Parse(time.Duration(int64(minimum))*time.Second))
	fmt.Printf("Maximum: %s\n", durafmt.Parse(time.Duration(int64(maximum))*time.Second))
	if shortest != nil {
		fmt.Printf("Shortest PR\n\tTitle: %s\n\tURL: %s\n", *shortest.PullRequest.Title, *shortest.PullRequest.URL)
	}
	if longest != nil {
		fmt.Printf("Longest PR\n\tTitle: %s\n\tURL: %s\n", *longest.PullRequest.Title, *longest.PullRequest.URL)
	}
}
