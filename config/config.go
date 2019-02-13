package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sigs.k8s.io/yaml"
	"strings"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

// Config is a read-only snapshot of the config.
type Config struct {
	JobConfig
	ProwConfig
}

// JobConfig is config for all prow jobs
type JobConfig struct {
	// Presets apply to all job types.
	Presets []Preset `json:"presets,omitempty"`
	// Full repo name (such as "kubernetes/kubernetes") -> list of jobs.
	//Presubmits  map[string][]Presubmit  `json:"presubmits,omitempty"`
	//Postsubmits map[string][]Postsubmit `json:"postsubmits,omitempty"`

	// Periodics are not associated with any repo.
	//Periodics []Periodic `json:"periodics,omitempty"`
}

// ProwConfig is config for all prow controllers
type ProwConfig struct {
	//Tide             Tide                  `json:"tide,omitempty"`
	//Plank            Plank                 `json:"plank,omitempty"`
	//Sinker           Sinker                `json:"sinker,omitempty"`
	//Deck             Deck                  `json:"deck,omitempty"`
	//BranchProtection BranchProtection      `json:"branch-protection,omitempty"`
	//Orgs             map[string]org.Config `json:"orgs,omitempty"`
	//Gerrit           Gerrit                `json:"gerrit,omitempty"`

	// TODO: Move this out of the main config.
	//JenkinsOperators []JenkinsOperator `json:"jenkins_operators,omitempty"`

	// ProwJobNamespace is the namespace in the cluster that prow
	// components will use for looking up ProwJobs. The namespace
	// needs to exist and will not be created by prow.
	// Defaults to "default".
	ProwJobNamespace string `json:"prowjob_namespace,omitempty"`
	// PodNamespace is the namespace in the cluster that prow
	// components will use for looking up Pods owned by ProwJobs.
	// The namespace needs to exist and will not be created by prow.
	// Defaults to "default".
	PodNamespace string `json:"pod_namespace,omitempty"`

	// LogLevel enables dynamically updating the log level of the
	// standard logger that is used by all prow components.
	//
	// Valid values:
	//
	// "debug", "info", "warn", "warning", "error", "fatal", "panic"
	//
	// Defaults to "info".
	//LogLevel string `json:"log_level,omitempty"`

	// PushGateway is a prometheus push gateway.
	//PushGateway PushGateway `json:"push_gateway,omitempty"`

	// OwnersDirBlacklist is used to configure which directories to ignore when
	// searching for OWNERS{,_ALIAS} files in a repo.
	OwnersDirBlacklist OwnersDirBlacklist `json:"owners_dir_blacklist,omitempty"`

	// Pub/Sub Subscriptions that we want to listen to
	//PubSubSubscriptions PubsubSubscriptions `json:"pubsub_subscriptions,omitempty"`
}

// OwnersDirBlacklist is used to configure which directories to ignore when
// searching for OWNERS{,_ALIAS} files in a repo.
type OwnersDirBlacklist struct {
	// Repos configures a directory blacklist per repo (or org)
	Repos map[string][]string `json:"repos"`
	// Default configures a default blacklist for repos (or orgs) not
	// specifically configured
	Default []string `json:"default"`
}

// Load loads and parses the config at path.
func Load(prowConfig, jobConfig string) (c *Config, err error) {
	// we never want config loading to take down the prow components
	defer func() {
		if r := recover(); r != nil {
			c, err = nil, fmt.Errorf("panic loading config: %v", r)
		}
	}()
	c, err = loadConfig(prowConfig, jobConfig)
	if err != nil {
		return nil, err
	}
	//if err := c.finalizeJobConfig(); err != nil {
	//	return nil, err
	//}
	//if err := c.validateComponentConfig(); err != nil {
	//	return nil, err
	//}
	//if err := c.validateJobConfig(); err != nil {
	//	return nil, err
	//}
	return c, nil
}

// loadConfig loads one or multiple config files and returns a config object.
func loadConfig(prowConfig, jobConfig string) (*Config, error) {
	fmt.Println("&&&&&& prow config &&&&&&&&", prowConfig)
	fmt.Println("$$$$$$$ jobconfig $$$$$$$$$$$", jobConfig)
	stat, err := os.Stat(prowConfig)
	if err != nil {
		return nil, err
	}

	if stat.IsDir() {
		return nil, fmt.Errorf("prowConfig cannot be a dir - %s", prowConfig)
	}

	var nc Config
	fmt.Println("^^^^^^^^^^Config^^^^^^^^^", nc.ProwConfig)
	if err := yamlToConfig(prowConfig, &nc); err != nil {
		return nil, err
	}
	//if err := parseProwConfig(&nc); err != nil {
	//	return nil, err
	//}

	// TODO(krzyzacy): temporary allow empty jobconfig
	//                 also temporary allow job config in prow config
	if jobConfig == "" {
		return &nc, nil
	}

	stat, err = os.Stat(jobConfig)
	if err != nil {
		return nil, err
	}

	if !stat.IsDir() {
		// still support a single file
		var jc JobConfig
		if err := yamlToConfig(jobConfig, &jc); err != nil {
			return nil, err
		}
		if err := nc.mergeJobConfig(jc); err != nil {
			return nil, err
		}
		return &nc, nil
	}

	// we need to ensure all config files have unique basenames,
	// since updateconfig plugin will use basename as a key in the configmap
	uniqueBasenames := sets.String{}

	err = filepath.Walk(jobConfig, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logrus.WithError(err).Errorf("walking path %q.", path)
			// bad file should not stop us from parsing the directory
			return nil
		}

		if strings.HasPrefix(info.Name(), "..") {
			// kubernetes volumes also include files we
			// should not look be looking into for keys
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if filepath.Ext(path) != ".yaml" && filepath.Ext(path) != ".yml" {
			return nil
		}

		if info.IsDir() {
			return nil
		}

		base := filepath.Base(path)
		if uniqueBasenames.Has(base) {
			return fmt.Errorf("duplicated basename is not allowed: %s", base)
		}
		uniqueBasenames.Insert(base)

		var subConfig JobConfig
		if err := yamlToConfig(path, &subConfig); err != nil {
			return err
		}
		return nc.mergeJobConfig(subConfig)
	})

	if err != nil {
		return nil, err
	}

	return &nc, nil
}

// yamlToConfig converts a yaml file into a Config object
func yamlToConfig(path string, nc interface{}) error {
	fmt.Println("****** nc ***********", nc)
	b, err := ioutil.ReadFile(path)
	fmt.Println("&&&& b &&&&&&&&&", string(b))
	if err != nil {
		return fmt.Errorf("error reading %s: %v", path, err)
	}
	if err := yaml.Unmarshal(b, nc); err != nil {
		return fmt.Errorf("error unmarshaling %s: %v", path, err)
	}
	fmt.Println("&&&&&&& nc &&&&&&&&&&", nc)
	var jc *JobConfig
	fmt.Print("jc", jc)
	switch v := nc.(type) {
	case *JobConfig:
		jc = v
	case *Config:
		jc = &v.JobConfig
	}
	//for rep := range jc.Presubmits {
	//	var fix func(*Presubmit)
	//	fix = func(job *Presubmit) {
	//		job.SourcePath = path
	//		for i := range job.RunAfterSuccess {
	//			fix(&job.RunAfterSuccess[i])
	//		}
	//	}
	//	for i := range jc.Presubmits[rep] {
	//		fix(&jc.Presubmits[rep][i])
	//	}
	//}
	//for rep := range jc.Postsubmits {
	//	var fix func(*Postsubmit)
	//	fix = func(job *Postsubmit) {
	//		job.SourcePath = path
	//		for i := range job.RunAfterSuccess {
	//			fix(&job.RunAfterSuccess[i])
	//		}
	//	}
	//	for i := range jc.Postsubmits[rep] {
	//		fix(&jc.Postsubmits[rep][i])
	//	}
	//}
	//
	//var fix func(*Periodic)
	//fix = func(job *Periodic) {
	//	job.SourcePath = path
	//	for i := range job.RunAfterSuccess {
	//		fix(&job.RunAfterSuccess[i])
	//	}
	//}
	//for i := range jc.Periodics {
	//	fix(&jc.Periodics[i])
	//}
	return nil
}

// mergeConfig merges two JobConfig together
// It will try to merge:
//	- Presubmits
//	- Postsubmits
// 	- Periodics
//	- PodPresets
func (c *Config) mergeJobConfig(jc JobConfig) error {
	// Merge everything
	// *** Presets ***
	c.Presets = append(c.Presets, jc.Presets...)

	// validate no duplicated preset key-value pairs
	validLabels := map[string]bool{}
	for _, preset := range c.Presets {
		for label, val := range preset.Labels {
			pair := label + ":" + val
			if _, ok := validLabels[pair]; ok {
				return fmt.Errorf("duplicated preset 'label:value' pair : %s", pair)
			}
			validLabels[pair] = true
		}
	}

	// *** Periodics ***
	//c.Periodics = append(c.Periodics, jc.Periodics...)
	//
	//// *** Presubmits ***
	//if c.Presubmits == nil {
	//	c.Presubmits = make(map[string][]Presubmit)
	//}
	//for repo, jobs := range jc.Presubmits {
	//	c.Presubmits[repo] = append(c.Presubmits[repo], jobs...)
	//}
	//
	//// *** Postsubmits ***
	//if c.Postsubmits == nil {
	//	c.Postsubmits = make(map[string][]Postsubmit)
	//}
	//for repo, jobs := range jc.Postsubmits {
	//	c.Postsubmits[repo] = append(c.Postsubmits[repo], jobs...)
	//}
	//
	return nil
}
