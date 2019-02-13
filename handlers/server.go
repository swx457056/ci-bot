package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/google/go-github/github"
	"github.com/spf13/pflag"
	"golang.org/x/oauth2"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
)

// Server implements http.Handler. It validates incoming GitHub webhooks and
// then dispatches them to the handlers accordingly.
type Server struct {
	Config       Config
	GithubClient *github.Client
	Context      context.Context
}

type Config struct {
	Owner         string `json:"owner"`
	Repo          string `json:"repo"`
	GitHubToken   string `json:"git_hub_token"`
	WebhookSecret string `json:"webhook_secret"`
	CircleCIToken string `json:"circle_ci_token"`
}

type WebHookServer struct {
	Address    string
	Port       int64
	ConfigFile string
}

func NewWebHookServer() *WebHookServer {
	s := WebHookServer{
		Address:    "0.0.0.0",
		Port:       3000,
		ConfigFile: "/root/bot/src/ci-bot/config.json",
	}
	return &s
}

func AddFlags(fs *pflag.FlagSet, s *WebHookServer) {
	fs.StringVar(&s.Address, "address", s.Address, "IP address to serve, 0.0.0.0 by default")
	fs.Int64Var(&s.Port, "port", s.Port, "Port to listen on, 3000 by default")
	fs.StringVar(&s.ConfigFile, "config-file", s.ConfigFile, "Config file.")
}

// ServeHTTP validates an incoming webhook and invoke its handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	payload, err := github.ValidatePayload(r, []byte(s.Config.WebhookSecret))
	if err != nil {
		glog.Errorf("Invalid payload: %v", err)
		return
	}
	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		glog.Errorf("Failed to parse webhook")
		return
	}
	fmt.Fprint(w, "Received a webhook event")

	//glog.Infof("body: %v", string(payload))

	var client http.Client
	client.Do(r)
	switch event.(type) {
	case *github.IssueEvent:

		go s.handleIssueEvent(payload)
	case *github.IssueCommentEvent:
		// Comments on PRs belong to IssueCommentEvent
		go s.handleIssueCommentEvent(payload, ClientRepo)
	case *github.PullRequestEvent:
		go s.handlePullRequestEvent(payload, ClientRepo)
	case *github.PullRequestComment:
		go s.handlePullRequestCommentEvent(payload)
	}
}

var ClientRepo *github.Client

func Run(s *WebHookServer) {
	configContent, err := ioutil.ReadFile(s.ConfigFile)
	if err != nil {
		glog.Fatal("Could not read config file: %v", err)
	}
	var config Config
	err = json.Unmarshal(configContent, &config)
	if err != nil {
		glog.Fatal("fail to unmarshal: %v", err)
	}
	oauthSecret := config.GitHubToken
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: string(oauthSecret)},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)
	ClientRepo = client
	webHookHandler := Server{
		Config:       config,
		GithubClient: ClientRepo,
		Context:      ctx,
	}
	//setting handler
	http.HandleFunc("/hook", webHookHandler.ServeHTTP)

	address := s.Address + ":" + strconv.FormatInt(s.Port, 10)
	//starting server
	if err := http.ListenAndServe(address, nil); err != nil {
		log.Println(err)
	}
}
