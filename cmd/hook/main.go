/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/golang/glog"
	"github.com/google/go-github/github"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"

	"ci-bot/config"
	"ci-bot/hook"
	pluginhelp "ci-bot/pluginhelp/hook"
	"ci-bot/plugins"
	_ "ci-bot/plugins/assign"
)

type options struct {
	port         int
	Address      string
	configPath   string
	pluginConfig string
	gracePeriod  time.Duration
}

func gatherOptions() options {
	o := options{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fs.IntVar(&o.port, "port", 3000, "Port to listen on.")
	fs.StringVar(&o.configPath, "config-path", "/root/bot/src/ci-bot-test/ci-bot/config.json", "Path to config.yaml.")
	fs.StringVar(&o.pluginConfig, "plugin-config", "/root/bot/src/ci-bot-test/ci-bot/plugins.yaml", "Path to plugin config file.")

	return o
}

func main() {
	o := gatherOptions()
	configAgent := &config.Agent{}
	configContent, err := ioutil.ReadFile(o.configPath)
	var config hook.Config

	err = json.Unmarshal(configContent, &config)
	if err != nil {
		glog.Fatal("fail to unmarshal: %v", err)
	}

	/*oauthSecret := config.GitHubToken
	fmt.Println("oauth sercret", oauthSecret)
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: string(oauthSecret)},
	)

	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)
	fmt.Println("client", client)*/

	r := bufio.NewReader(os.Stdin)
	fmt.Print("GitHub Username: ")
	username, _ := r.ReadString('\n')

	fmt.Print("GitHub Password: ")
	bytePassword, _ := terminal.ReadPassword(int(syscall.Stdin))
	password := string(bytePassword)

	tp := github.BasicAuthTransport{
		Username: strings.TrimSpace(username),
		Password: strings.TrimSpace(password),
	}

	client := github.NewClient(tp.Client())
	ctx := context.Background()
	user, _, err := client.Users.Get(ctx, "")
	fmt.Println("user", user)
	// Is this a two-factor auth error? If so, prompt for OTP and try again.
	if _, ok := err.(*github.TwoFactorAuthError); ok {
		fmt.Print("\nGitHub OTP: ")
		otp, _ := r.ReadString('\n')
		tp.OTP = strings.TrimSpace(otp)
	}

	if err != nil {
		return
	}

	// Return 200 on / for health checks.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {})

	pluginAgent := &plugins.ConfigAgent{}
	if err := pluginAgent.Start(o.pluginConfig); err != nil {
		logrus.WithError(err).Fatal("Error starting plugins.")
	}
	getSecret := func() []byte {
		return []byte(config.WebhookSecret)
	}
	server := &hook.Server{
		GithubClient: client,
		ConfigAgent:  configAgent,
		Plugins:      pluginAgent,
		Config:       config,
		Context:      context.Background(),
		TokenGenerator: getSecret,
	}
	defer server.GracefulShutdown()
	// For /hook, handle a webhook normally.
	http.Handle("/hook", server)
	// Serve plugin help information from /plugin-help.

	http.Handle("/plugin-help", pluginhelp.NewHelpAgent(pluginAgent, client))

	httpServer := &http.Server{Addr: ":" + strconv.Itoa(o.port)}

	// Shutdown gracefully on SIGTERM or SIGINT
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		logrus.Info("Hook is shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), o.gracePeriod)
		defer cancel()
		httpServer.Shutdown(ctx)
	}()

	logrus.WithError(httpServer.ListenAndServe()).Warn("Server exited.")
}
