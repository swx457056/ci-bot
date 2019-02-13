package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/golang/glog"
	"github.com/google/go-github/github"
)

type GithubIssue github.Issue

func (s *Server) handleIssueEvent(body []byte) {
	glog.Infof("Received an Issue Event")

}

func (s *Server) handleIssueCommentEvent(body []byte, client *github.Client) {
	glog.Infof("Received an IssueComment Event")

	var prc github.IssueCommentEvent
	err := json.Unmarshal(body, &prc)
	if err != nil {
		glog.Errorf("fail to unmarshal: %v", err)
	}
	glog.Infof("prc: %v", prc)

	owner := *prc.Repo.Owner.Login

	ctx := context.Background()
	user := prc.Sender.Login
	list, _, err := client.Repositories.ListCollaborators(ctx, owner, *prc.Repo.Name, nil)
	fmt.Println("list", list)
	if err != nil {
		glog.Fatal("Cannot List the Collaborators", err)
	}

	comment := *prc.Comment.Body

	assignees := strings.TrimPrefix(comment, "/assign @")
	assign, _, err := client.Repositories.IsCollaborator(ctx, owner, *prc.Repo.Name, *user)
	if err != nil {
		glog.Fatal("Not the collaborator", err)

	}
	reg := regexp.MustCompile("(?mi)^/(un)?assign(( @?[-\\w]+?)*)\\s*$")
	comm := reg.MatchString(comment)
	get := make([]string, 0)
	get = append(get, assignees)

	if assign {
		if comm == true {
			issue, _, err := client.Issues.AddAssignees(ctx, *prc.Repo.Owner.Login, *prc.Repo.Name, *prc.Issue.Number, get)
			if err != nil {
				glog.Fatal("Cannot Add Assignees", err)
			}
			if issue != nil {
				fmt.Println("Assignee added successfully")
			}

		}

	}

}
