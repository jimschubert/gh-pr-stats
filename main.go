package main

import (
	"context"
	"fmt"
	"gh-pr-stats/model"
	"github.com/google/go-github/v29/github"
	"github.com/hako/durafmt"
	"github.com/jszwec/csvutil"
	"github.com/montanaflynn/stats"
	"os"
	"strings"
	"time"
)
import "golang.org/x/oauth2"
import "github.com/jessevdk/go-flags"

var version = ""
var date = ""
var commit = ""
var projectName = ""

var opts struct {
	Owner string `short:"o" long:"owner" description:"GitHub Owner/Org name (required)" required:"true"`

	Repo string `short:"r" long:"repo" description:"GitHub Repo name (required)" required:"true"`

	Start string `short:"s" long:"start" description:"Start date in format YYYY-mm-dd"`

	End string `short:"e" long:"end" description:"End date in format YYYY-mm-dd"`

	Enterprise string `long:"enterprise" description:"GitHub Enterprise URL in the format http(s)://[hostname]/api/v3"`

	CSV bool `long:"csv" description:"Dump the full pr details output to stdout as CSV for stats processing by other tools"`

	Verbose bool `long:"verbose" description:"Display verbose messages"`

	Version bool `short:"v" long:"version" description:"Display version information"`
}

var start time.Time
var end time.Time

func newContext(c context.Context) (context.Context, context.CancelFunc) {
	 timeout, cancel := context.WithTimeout(c, 15 * time.Second)
	 return timeout, cancel
}

func retrievePullRequests(client *github.Client, pullOpts *github.PullRequestListOptions, prStats *model.PullRequestStats, details *[]*model.PullRequestDetails) (int, error) {
	ctx, cancel := newContext(context.Background())
	defer cancel()
	pulls, _, err := client.PullRequests.List(ctx, opts.Owner, opts.Repo, pullOpts)
	if err != nil {
		return 0, err
	}

	durations := make([]int64, 0)
	for _, pull := range pulls {
		if start.Unix() < pull.CreatedAt.Unix() && pull.CreatedAt.Unix() < end.Unix() {
			if opts.CSV {
				pr := model.PullRequestDetails{}
				pr.FromGitHubPullRequest(pull)
				*details = append(*details, &pr)
			}

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

			if prStats.Shortest.PullRequest == nil || f < prStats.Shortest.Duration {
				prStats.Shortest.Update(pull, f)
			}
			if prStats.Longest.PullRequest == nil || prStats.Longest.Duration < f {
				prStats.Longest.Update(pull, f)
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

const parseArgs = flags.HelpFlag | flags.PassDoubleDash

func main() {
	parser := flags.NewParser(&opts, parseArgs)
	args, err := parser.Parse()
	if err != nil {
		flagError := err.(*flags.Error)
		if flagError.Type == flags.ErrHelp {
			parser.WriteHelp(os.Stdout)
			return
		}

		if flagError.Type == flags.ErrUnknownFlag {
			_, _ = fmt.Fprintf(os.Stderr, "%s. Please use --help for available options.\n", strings.Replace(flagError.Message, "unknown", "Unknown", 1))
			return
		}
		_, _ = fmt.Fprintf(os.Stderr, "Error parsing command line options: %s\n", err)
		return
	}

	if opts.Version {
		fmt.Printf("%s %s (%s)\n", projectName, version, commit)
		return
	}

	// only accept switches, no args
	if len(args) > 0 {
		_, _ = fmt.Fprintf(os.Stderr, "Unknown command line argument '%s'.\n", args[0])
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

	prStats := model.NewPullRequestStats()

	details := make([]*model.PullRequestDetails, 0)
	for {
		count, err := retrievePullRequests(client, pullOpts, prStats, &details)

		if err != nil {
			fmt.Printf("Error retrieving pull requests '%s.\n'", err)
			os.Exit(1)
		}

		if count <= 0 {
			break
		}

		pullOpts.Page = pullOpts.Page + 1
	}

	if opts.CSV {
		b, err := csvutil.Marshal(details)
		if err != nil {
			fmt.Println("error:", err)
		}
		fmt.Println(string(b))
	} else {
		data := stats.LoadRawData(prStats.Durations)
		minimum, _ := stats.Min(data)
		median, _ := stats.Median(data)
		maximum, _ := stats.Max(data)

		fmt.Printf("Pull Requests:\n\tOpen: %d\n\tClosed: %d\n", prStats.OpenCount, prStats.ClosedCount)
		fmt.Printf("Open Duration:\n")
		fmt.Printf("\tMinimum: %s\n", durafmt.Parse(time.Duration(int64(minimum))*time.Second))
		fmt.Printf("\tMedian: %s\n", durafmt.Parse(time.Duration(int64(median))*time.Second))
		fmt.Printf("\tMaximum: %s\n", durafmt.Parse(time.Duration(int64(maximum))*time.Second))

		if prStats.Shortest.PullRequest != nil {
			fmt.Printf("Shortest-lived PR:\n\tTitle: %s\n\tURL: %s\n\tAuthor: %s\n", *prStats.Shortest.PullRequest.Title, *prStats.Shortest.PullRequest.HTMLURL, prStats.Shortest.PullRequest.GetUser().GetLogin())
		}
		if prStats.Longest.PullRequest != nil {
			fmt.Printf("Longest-lived PR:\n\tTitle: %s\n\tURL: %s\n\tAuthor: %s\n", *prStats.Longest.PullRequest.Title, *prStats.Longest.PullRequest.HTMLURL, prStats.Longest.PullRequest.GetUser().GetLogin())
		}
	}
}
