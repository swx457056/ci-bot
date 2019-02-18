/*
Copyright 2018 The Kubernetes Authors.

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

package plugins

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	defaultBlunderbussReviewerCount = 2
)

type Blockade struct {
	// Repos are either of the form org/repos or just org.
	Repos []string `json:"repos,omitempty"`
	// BlockRegexps are regular expressions matching the file paths to block.
	BlockRegexps []string `json:"blockregexps,omitempty"`
	// ExceptionRegexps are regular expressions matching the file paths that are exceptions to the BlockRegexps.
	ExceptionRegexps []string `json:"exceptionregexps,omitempty"`
	// Explanation is a string that will be included in the comment left when blocking a PR. This should
	// be an explanation of why the paths specified are blockaded.
	Explanation string `json:"explanation,omitempty"`
}

// Size specifies configuration for the size plugin, defining lower bounds (in # lines changed) for each size label.
// XS is assumed to be zero.
type Size struct {
	S   int `json:"s"`
	M   int `json:"m"`
	L   int `json:"l"`
	Xl  int `json:"xl"`
	Xxl int `json:"xxl"`
}

// Blunderbuss defines configuration for the blunderbuss plugin.
type Blunderbuss struct {
	// ReviewerC
	// // Cat contains the configuration for the cat plugin.
	//type Cat struct {
	//	// Path to file containing an api key for thecatapi.com
	//	KeyPath string `json:"key_path,omitempty"`
	//}ount is the minimum number of reviewers to request
	// reviews from. Defaults to requesting reviews from 2 reviewers
	// if FileWeightCount is not set.
	ReviewerCount *int `json:"request_count,omitempty"`
	// MaxReviewerCount is the maximum number of reviewers to request
	// reviews from. Defaults to 0 meaning no limit.
	MaxReviewerCount int `json:"max_request_count,omitempty"`
	// FileWeightCount is the maximum number of reviewers to request
	// reviews from. Selects reviewers based on file weighting.
	// This and request_count are mutually exclusive options.
	FileWeightCount *int `json:"file_weight_count,omitempty"`
	// ExcludeApprovers controls whether approvers are considered to be
	// reviewers. By default, approvers are considered as reviewers if
	// insufficient reviewers are available. If ExcludeApprovers is true,
	// approvers will never be considered as reviewers.
	ExcludeApprovers bool `json:"exclude_approvers,omitempty"`
}

// RequireMatchingLabel is the config for the require-matching-label plugin.
type RequireMatchingLabel struct {
	// Org is the GitHub organization that this config applies to.
	Org string `json:"org,omitempty"`
	// Repo is the GitHub repository within Org that this config applies to.
	// This fields may be omitted to apply this config across all repos in Org.
	Repo string `json:"repo,omitempty"`
	// Branch is the branch ref of PRs that this config applies to.
	// This field is only valid if `prs: true` and may be omitted to apply this
	// config across all branches in the repo or org.
	Branch string `json:"branch,omitempty"`
	// PRs is a bool indicating if this config applies to PRs.
	PRs bool `json:"prs,omitempty"`
	// Issues is a bool indicating if this config applies to issues.
	Issues bool `json:"issues,omitempty"`

	// Regexp is the string specifying the regular expression used to look for
	// matching labels.
	Regexp string `json:"regexp,omitempty"`
	// Re is the compiled version of Regexp. It should not be specified in config.
	Re *regexp.Regexp `json:"-"`

	// MissingLabel is the label to apply if an issue does not have any label
	// matching the Regexp.
	MissingLabel string `json:"missing_label,omitempty"`
	// MissingComment is the comment to post when we add the MissingLabel to an
	// issue. This is typically used to explain why MissingLabel was added and
	// how to move forward.
	// This field is optional. If unspecified, no comment is created when labeling.
	MissingComment string `json:"missing_comment,omitempty"`

	// GracePeriod is the amount of time to wait before processing newly opened
	// or reopened issues and PRs. This delay allows other automation to apply
	// labels before we look for matching labels.
	// Defaults to '5s'.
	GracePeriod         string        `json:"grace_period,omitempty"`
	GracePeriodDuration time.Duration `json:"-"`
}

// Milestone contains the configuration options for the milestone and
// milestonestatus plugins.
type Milestone struct {
	// ID of the github team for the milestone maintainers (used for setting status labels)
	// You can curl the following endpoint in order to determine the github ID of your team
	// responsible for maintaining the milestones:
	// curl -H "Authorization: token <token>" https://api.github.com/orgs/<org-name>/teams
	MaintainersID           int    `json:"maintainers_id,omitempty"`
	MaintainersTeam         string `json:"maintainers_team,omitempty"`
	MaintainersFriendlyName string `json:"maintainers_friendly_name,omitempty"`
}

// Lgtm specifies a configuration for a single lgtm.
// The configuration for the lgtm plugin is defined as a list of these structures.
type Lgtm struct {
	// Repos is either of the form org/repos or just org.
	Repos []string `json:"repos,omitempty"`
	// ReviewActsAsLgtm indicates that a Github review of "approve" or "request changes"
	// acts as adding or removing the lgtm label
	ReviewActsAsLgtm bool `json:"review_acts_as_lgtm,omitempty"`
	// StoreTreeHash indicates if tree_hash should be stored inside a comment to detect
	// squashed commits before removing lgtm labels
	StoreTreeHash bool `json:"store_tree_hash,omitempty"`
	// WARNING: This disables the security mechanism that prevents a malicious member (or
	// compromised GitHub account) from merging arbitrary code. Use with caution.
	//
	// StickyLgtmTeam specifies the Github team whose members are trusted with sticky LGTM,
	// which eliminates the need to re-lgtm minor fixes/updates.
	StickyLgtmTeam string `json:"trusted_team_for_sticky_lgtm,omitempty"`
}

// Label contains the configuration for the label plugin.
type Label struct {
	// AdditionalLabels is a set of additional labels enabled for use
	// on top of the existing "kind/*", "priority/*", and "area/*" labels.
	AdditionalLabels []string `json:"additional_labels"`
}

// Heart contains the configuration for the heart plugin.
type Heart struct {
	// Adorees is a list of GitHub logins for members
	// for whom we will add emojis to comments
	Adorees []string `json:"adorees,omitempty"`
	// CommentRegexp is the regular expression for comments
	// made by adorees that the plugin adds emojis to.
	// If not specified, the plugin will not add emojis to
	// any comments.
	// Compiles into CommentRe during config load.
	CommentRegexp string         `json:"commentregexp,omitempty"`
	CommentRe     *regexp.Regexp `json:"-"`
}

// Golint holds configuration for the golint plugin
type Golint struct {
	// MinimumConfidence is the smallest permissible confidence
	// in (0,1] over which problems will be printed. Defaults to
	// 0.8, as does the `go lint` tool.
	MinimumConfidence *float64 `json:"minimum_confidence,omitempty"`
}

// ConfigMapSpec contains configuration options for the configMap being updated
// by the config-updater plugin.
type ConfigMapSpec struct {
	// Name of ConfigMap
	Name string `json:"name"`
	// Key is the key in the ConfigMap to update with the file contents.
	// If no explicit key is given, the basename of the file will be used.
	Key string `json:"key,omitempty"`
	// Namespace in which the configMap needs to be deployed. If no namespace is specified
	// it will be deployed to the ProwJobNamespace.
	Namespace string `json:"namespace,omitempty"`
	// Namespaces in which the configMap needs to be deployed, in addition to the above
	// namespace provided, or the default if it is not set.
	AdditionalNamespaces []string `json:"additional_namespaces,omitempty"`

	// Namespaces is the fully resolved list of Namespaces to deploy the ConfigMap in
	Namespaces []string `json:"-"`
}

// ConfigUpdater contains the configuration for the config-updater plugin.
type ConfigUpdater struct {
	// A map of filename => ConfigMapSpec.
	// Whenever a commit changes filename, prow will update the corresponding configmap.
	// map[string]ConfigMapSpec{ "/my/path.yaml": {Name: "foo", Namespace: "otherNamespace" }}
	// will result in replacing the foo configmap whenever path.yaml changes
	Maps map[string]ConfigMapSpec `json:"maps,omitempty"`
	// The location of the prow configuration file inside the repository
	// where the config-updater plugin is enabled. This needs to be relative
	// to the root of the repository, eg. "prow/config.yaml" will match
	// github.com/kubernetes/test-infra/prow/config.yaml assuming the config-updater
	// plugin is enabled for kubernetes/test-infra. Defaults to "prow/config.yaml".
	ConfigFile string `json:"config_file,omitempty"`
	// The location of the prow plugin configuration file inside the repository
	// where the config-updater plugin is enabled. This needs to be relative
	// to the root of the repository, eg. "prow/plugins.yaml" will match
	// github.com/kubernetes/test-infra/prow/plugins.yaml assuming the config-updater
	// plugin is enabled for kubernetes/test-infra. Defaults to "prow/plugins.yaml".
	PluginFile string `json:"plugin_file,omitempty"`
}

// CherryPickUnapproved is the config for the cherrypick-unapproved plugin.
type CherryPickUnapproved struct {
	// BranchRegexp is the regular expression for branch names such that
	// the plugin treats only PRs against these branch names as cherrypick PRs.
	// Compiles into BranchRe during config load.
	BranchRegexp string         `json:"branchregexp,omitempty"`
	BranchRe     *regexp.Regexp `json:"-"`
	// Comment is the comment added by the plugin while adding the
	// `do-not-merge/cherry-pick-not-approved` label.
	Comment string `json:"comment,omitempty"`
}

// Owners contains configuration related to handling OWNERS files.
type Owners struct {
	// MDYAMLRepos is a list of org and org/repo strings specifying the repos that support YAML
	// OWNERS config headers at the top of markdown (*.md) files. These headers function just like
	// the config in an OWNERS file, but only apply to the file itself instead of the entire
	// directory and all sub-directories.
	// The yaml header must be at the start of the file and be bracketed with "---" like so:
	/*
		---
		approvers:
		- mikedanese
		- thockin

		---
	*/
	MDYAMLRepos []string `json:"mdyamlrepos,omitempty"`

	// SkipCollaborators disables collaborator cross-checks and forces both
	// the approve and lgtm plugins to use solely OWNERS files for access
	// control in the provided repos.
	SkipCollaborators []string `json:"skip_collaborators,omitempty"`

	// LabelsBlackList holds a list of labels that should not be present in any
	// OWNERS file, preventing their automatic addition by the owners-label plugin.
	// This check is performed by the verify-owners plugin.
	LabelsBlackList []string `json:"labels_blacklist,omitempty"`
}

// Configuration is the top-level serialization target for plugin Configuration.
type Configuration struct {
	// Plugins is a map of repositories (eg "k/k") to lists of
	// plugin names.
	// TODO: Link to the list of supported plugins.
	// https://github.com/kubernetes/test-infra/issues/3476
	Plugins map[string][]string `json:"plugins,omitempty"`

	// ExternalPlugins is a map of repositories (eg "k/k") to lists of
	// external plugins.
	ExternalPlugins map[string][]ExternalPlugin `json:"external_plugins,omitempty"`

	// Owners contains configuration related to handling OWNERS files.
	Owners Owners `json:"owners,omitempty"`

	// Built-in plugins specific configuration.
	Approve                    []Approve   `json:"approve,omitempty"`
	UseDeprecatedSelfApprove   bool        `json:"use_deprecated_2018_implicit_self_approve_default_migrate_before_july_2019,omitempty"`
	UseDeprecatedReviewApprove bool        `json:"use_deprecated_2018_review_acts_as_approve_default_migrate_before_july_2019,omitempty"`
	Blockades                  []Blockade  `json:"blockades,omitempty"`
	Blunderbuss                Blunderbuss `json:"blunderbuss,omitempty"`
	//Cat                        Cat                    `json:"cat,omitempty"`
	CherryPickUnapproved CherryPickUnapproved   `json:"cherry_pick_unapproved,omitempty"`
	ConfigUpdater        ConfigUpdater          `json:"config_updater,omitempty"`
	Golint               *Golint                `json:"golint,omitempty"`
	Heart                Heart                  `json:"heart,omitempty"`
	Label                *Label                 `json:"label,omitempty"`
	Lgtm                 []Lgtm                 `json:"lgtm,omitempty"`
	RepoMilestone        map[string]Milestone   `json:"repo_milestone,omitempty"`
	RequireMatchingLabel []RequireMatchingLabel `json:"require_matching_label,omitempty"`
	//RequireSIG                 RequireSIG             `json:"requiresig,omitempty"`
	//Slack                      Slack                  `json:"slack,omitempty"`
	SigMention SigMention `json:"sigmention,omitempty"`
	//Size                       *Size                  `json:"size,omitempty"`
	Triggers []Trigger `json:"triggers,omitempty"`
	//Welcome                    []Welcome              `json:"welcome,omitempty"`
}

// SigMention specifies configuration for the sigmention plugin.
type SigMention struct {
	// Regexp parses comments and should return matches to team mentions.
	// These mentions enable labeling issues or PRs with sig/team labels.
	// Furthermore, teams with the following suffixes will be mapped to
	// kind/* labels:
	//
	// * @org/team-bugs             --maps to--> kind/bug
	// * @org/team-feature-requests --maps to--> kind/feature
	// * @org/team-api-reviews      --maps to--> kind/api-change
	// * @org/team-proposals        --maps to--> kind/design
	//
	// Note that you need to make sure your regexp covers the above
	// mentions if you want to use the extra labeling. Defaults to:
	// (?m)@kubernetes/sig-([\w-]*)-(misc|test-failures|bugs|feature-requests|proposals|pr-reviews|api-reviews)
	//
	// Compiles into Re during config load.
	Regexp string         `json:"regexp,omitempty"`
	Re     *regexp.Regexp `json:"-"`
}

// Trigger specifies a configuration for a single trigger.
//
// The configuration for the trigger plugin is defined as a list of these structures.
type Trigger struct {
	// Repos is either of the form org/repos or just org.
	Repos []string `json:"repos,omitempty"`
	// TrustedOrg is the org whose members' PRs will be automatically built
	// for PRs to the above repos. The default is the PR's org.
	TrustedOrg string `json:"trusted_org,omitempty"`
	// JoinOrgURL is a link that redirects users to a location where they
	// should be able to read more about joining the organization in order
	// to become trusted members. Defaults to the Github link of TrustedOrg.
	JoinOrgURL string `json:"join_org_url,omitempty"`
	// OnlyOrgMembers requires PRs and/or /ok-to-test comments to come from org members.
	// By default, trigger also include repo collaborators.
	OnlyOrgMembers bool `json:"only_org_members,omitempty"`
	// IgnoreOkToTest makes trigger ignore /ok-to-test comments.
	// This is a security mitigation to only allow testing from trusted users.
	IgnoreOkToTest bool `json:"ignore_ok_to_test,omitempty"`
}

// ExternalPlugin holds configuration for registering an external
// plugin in prow.
type ExternalPlugin struct {
	// Name of the plugin.
	Name string `json:"name"`
	// Endpoint is the location of the external plugin. Defaults to
	// the name of the plugin, ie. "http://{{name}}".
	Endpoint string `json:"endpoint,omitempty"`
	// Events are the events that need to be demuxed by the hook
	// server to the external plugin. If no events are specified,
	// everything is sent.
	Events []string `json:"events,omitempty"`
}

// Approve specifies a configuration for a single approve.
//
// The configuration for the approve plugin is defined as a list of these structures.
type Approve struct {
	// Repos is either of the form org/repos or just org.
	Repos []string `json:"repos,omitempty"`
	// IssueRequired indicates if an associated issue is required for approval in
	// the specified repos.
	IssueRequired bool `json:"issue_required,omitempty"`

	// TODO(fejta): delete in June 2019
	DeprecatedImplicitSelfApprove *bool `json:"implicit_self_approve,omitempty"`
	// RequireSelfApproval requires PR authors to explicitly approve their PRs.
	// Otherwise the plugin assumes the author of the PR approves the changes in the PR.
	RequireSelfApproval *bool `json:"require_self_approval,omitempty"`

	// LgtmActsAsApprove indicates that the lgtm command should be used to
	// indicate approval
	LgtmActsAsApprove bool `json:"lgtm_acts_as_approve,omitempty"`

	// ReviewActsAsApprove should be replaced with its non-deprecated inverse: ignore_review_state.
	// TODO(fejta): delete in June 2019
	DeprecatedReviewActsAsApprove *bool `json:"review_acts_as_approve,omitempty"`
	// IgnoreReviewState causes the approve plugin to ignore the GitHub review state. Otherwise:
	// * an APPROVE github review is equivalent to leaving an "/approve" message.
	// * A REQUEST_CHANGES github review is equivalent to leaving an /approve cancel" message.
	IgnoreReviewState *bool `json:"ignore_review_state,omitempty"`
}

// SetDefaults sets default options for config updating
func (c *ConfigUpdater) SetDefaults() {
	if len(c.Maps) == 0 {
		cf := c.ConfigFile
		if cf == "" {
			cf = "prow/config.json"
		} else {
			logrus.Warnf(`config_file is deprecated, please switch to "maps": {"%s": "config"} before July 2018`, cf)
		}
		pf := c.PluginFile
		if pf == "" {
			pf = "prow/plugins.yaml"
		} else {
			logrus.Warnf(`plugin_file is deprecated, please switch to "maps": {"%s": "plugins"} before July 2018`, pf)
		}
		c.Maps = map[string]ConfigMapSpec{
			cf: {
				Name: "config",
			},
			pf: {
				Name: "plugins",
			},
		}
	}

	for name, spec := range c.Maps {
		spec.Namespaces = append([]string{spec.Namespace}, spec.AdditionalNamespaces...)
		c.Maps[name] = spec
	}
}

//func (c *Configuration) setDefaults() {
//	c.ConfigUpdater.SetDefaults()
//
//	for repo, plugins := range c.ExternalPlugins {
//		for i, p := range plugins {
//			if p.Endpoint != "" {
//				continue
//			}
//			c.ExternalPlugins[repo][i].Endpoint = fmt.Sprintf("http://%s", p.Name)
//		}
//	}
//	if c.Blunderbuss.ReviewerCount == nil && c.Blunderbuss.FileWeightCount == nil {
//		c.Blunderbuss.ReviewerCount = new(int)
//		*c.Blunderbuss.ReviewerCount = defaultBlunderbussReviewerCount
//	}
//	for i, trigger := range c.Triggers {
//		if trigger.TrustedOrg == "" || trigger.JoinOrgURL != "" {
//			continue
//		}
//		c.Triggers[i].JoinOrgURL = fmt.Sprintf("https://github.com/orgs/%s/people", trigger.TrustedOrg)
//	}
//	if c.SigMention.Regexp == "" {
//		c.SigMention.Regexp = `(?m)@kubernetes/sig-([\w-]*)-(misc|test-failures|bugs|feature-requests|proposals|pr-reviews|api-reviews)`
//	}
//	if c.Owners.LabelsBlackList == nil {
//		c.Owners.LabelsBlackList = []string{labels.Approved, labels.LGTM}
//	}
//	for _, milestone := range c.RepoMilestone {
//		if milestone.MaintainersFriendlyName == "" {
//			milestone.MaintainersFriendlyName = "SIG Chairs/TLs"
//		}
//	}
//	if c.CherryPickUnapproved.BranchRegexp == "" {
//		c.CherryPickUnapproved.BranchRegexp = `^release-.*$`
//	}
//	if c.CherryPickUnapproved.Comment == "" {
//		c.CherryPickUnapproved.Comment = `This PR is not for the master branch but does not have the ` + "`cherry-pick-approved`" + `  label. Adding the ` + "`do-not-merge/cherry-pick-not-approved`" + `  label.
//
//To approve the cherry-pick, please assign the patch release manager for the release branch by writing ` + "`/assign @username`" + ` in a comment when ready.
//
//The list of patch release managers for each release can be found [here](https://git.k8s.io/sig-release/release-managers.md).`
//	}
//
//	for i, rml := range c.RequireMatchingLabel {
//		if rml.GracePeriod == "" {
//			c.RequireMatchingLabel[i].GracePeriod = "5s"
//		}
//	}
//}

func (c *Configuration) setDefaults() {
	c.ConfigUpdater.SetDefaults()

	for repo, plugins := range c.ExternalPlugins {
		for i, p := range plugins {
			if p.Endpoint != "" {
				continue
			}
			c.ExternalPlugins[repo][i].Endpoint = fmt.Sprintf("http://%s", p.Name)
		}
	}
	if c.Blunderbuss.ReviewerCount == nil && c.Blunderbuss.FileWeightCount == nil {
		c.Blunderbuss.ReviewerCount = new(int)
		*c.Blunderbuss.ReviewerCount = defaultBlunderbussReviewerCount
	}
	for i, trigger := range c.Triggers {
		if trigger.TrustedOrg == "" || trigger.JoinOrgURL != "" {
			continue
		}
		c.Triggers[i].JoinOrgURL = fmt.Sprintf("https://github.com/orgs/%s/people", trigger.TrustedOrg)
	}
	if c.SigMention.Regexp == "" {
		c.SigMention.Regexp = `(?m)@kubernetes/sig-([\w-]*)-(misc|test-failures|bugs|feature-requests|proposals|pr-reviews|api-reviews)`
	}
	//if c.Owners.LabelsBlackList == nil {
	//	c.Owners.LabelsBlackList = []string{labels.Approved, labels.LGTM}
	//}
	for _, milestone := range c.RepoMilestone {
		if milestone.MaintainersFriendlyName == "" {
			milestone.MaintainersFriendlyName = "SIG Chairs/TLs"
		}
	}
	if c.CherryPickUnapproved.BranchRegexp == "" {
		c.CherryPickUnapproved.BranchRegexp = `^release-.*$`
	}
	if c.CherryPickUnapproved.Comment == "" {
		c.CherryPickUnapproved.Comment = `This PR is not for the master branch but does not have the ` + "`cherry-pick-approved`" + `  label. Adding the ` + "`do-not-merge/cherry-pick-not-approved`" + `  label.

To approve the cherry-pick, please assign the patch release manager for the release branch by writing ` + "`/assign @username`" + ` in a comment when ready.

The list of patch release managers for each release can be found [here](https://git.k8s.io/sig-release/release-managers.md).`
	}

	for i, rml := range c.RequireMatchingLabel {
		if rml.GracePeriod == "" {
			c.RequireMatchingLabel[i].GracePeriod = "5s"
		}
	}
}

func compileRegexpsAndDurations(pc *Configuration) error {
	cRe, err := regexp.Compile(pc.SigMention.Regexp)
	if err != nil {
		return err
	}
	pc.SigMention.Re = cRe

	branchRe, err := regexp.Compile(pc.CherryPickUnapproved.BranchRegexp)
	if err != nil {
		return err
	}
	pc.CherryPickUnapproved.BranchRe = branchRe

	commentRe, err := regexp.Compile(pc.Heart.CommentRegexp)
	if err != nil {
		return err
	}
	pc.Heart.CommentRe = commentRe

	rs := pc.RequireMatchingLabel
	for i := range rs {
		re, err := regexp.Compile(rs[i].Regexp)
		if err != nil {
			return fmt.Errorf("failed to compile label regexp: %q, error: %v", rs[i].Regexp, err)
		}
		rs[i].Re = re

		var dur time.Duration
		dur, err = time.ParseDuration(rs[i].GracePeriod)
		if err != nil {
			return fmt.Errorf("failed to compile grace period duration: %q, error: %v", rs[i].GracePeriod, err)
		}
		rs[i].GracePeriodDuration = dur
	}
	return nil
}

func findDuplicatedPluginConfig(repoConfig, orgConfig []string) []string {
	var dupes []string
	for _, repoPlugin := range repoConfig {
		for _, orgPlugin := range orgConfig {
			if repoPlugin == orgPlugin {
				dupes = append(dupes, repoPlugin)
			}
		}
	}

	return dupes
}

// validatePlugins will return error if
// there are unknown or duplicated plugins.
func validatePlugins(plugins map[string][]string) error {
	var errors []string
	for _, configuration := range plugins {
		for _, plugin := range configuration {
			if _, ok := pluginHelp[plugin]; !ok {
				errors = append(errors, fmt.Sprintf("unknown plugin: %s", plugin))
			}
		}
	}
	for repo, repoConfig := range plugins {
		if strings.Contains(repo, "/") {
			org := strings.Split(repo, "/")[0]
			if dupes := findDuplicatedPluginConfig(repoConfig, plugins[org]); len(dupes) > 0 {
				errors = append(errors, fmt.Sprintf("plugins %v are duplicated for %s and %s", dupes, repo, org))
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf(" invalid plugin configuration:\n\t%v", strings.Join(errors, "\n\t"))
	}
	return nil
}

func validateSizes(size *Size) error {
	if size == nil {
		return nil
	}

	if size.S > size.M || size.M > size.L || size.L > size.Xl || size.Xl > size.Xxl {
		return errors.New("invalid size plugin configuration - one of the smaller sizes is bigger than a larger one")
	}

	return nil
}

func validateExternalPlugins(pluginMap map[string][]ExternalPlugin) error {
	var errors []string

	for repo, plugins := range pluginMap {
		if !strings.Contains(repo, "/") {
			continue
		}
		org := strings.Split(repo, "/")[0]

		var orgConfig []string
		for _, p := range pluginMap[org] {
			orgConfig = append(orgConfig, p.Name)
		}

		var repoConfig []string
		for _, p := range plugins {
			repoConfig = append(repoConfig, p.Name)
		}

		if dupes := findDuplicatedPluginConfig(repoConfig, orgConfig); len(dupes) > 0 {
			errors = append(errors, fmt.Sprintf("external plugins %v are duplicated for %s and %s", dupes, repo, org))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("invalid plugin configuration:\n\t%v", strings.Join(errors, "\n\t"))
	}
	return nil
}

func (c *Configuration) Validate() error {
	if len(c.Plugins) == 0 {
		logrus.Warn("no plugins specified-- check syntax?")
	}

	// Defaulting should run before validation.
	c.setDefaults()
	// Regexp compilation should run after defaulting, but before validation.
	if err := compileRegexpsAndDurations(c); err != nil {
		return err
	}
	if err := validatePlugins(c.Plugins); err != nil {
		return err
	}
	if err := validateExternalPlugins(c.ExternalPlugins); err != nil {
		return err
	}
	//if err := validateBlunderbuss(&c.Blunderbuss); err != nil {
	//	return err
	//}
	//if err := validateConfigUpdater(&c.ConfigUpdater); err != nil {
	//	return err
	//}
	//if err := validateSizes(c.Size); err != nil {
	//	return err
	//}
	//if err := validateRequireMatchingLabel(c.RequireMatchingLabel); err != nil {
	//	return err
	//}

	return nil
}
