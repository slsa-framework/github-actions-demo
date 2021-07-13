package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

const (
	GitHubHostedIdSuffix = "/Attestations/GitHubHostedActions@v1"
	SelfHostedIdSuffix   = "/Attestations/SelfHostedActions@v1"
	TypeId               = "https://github.com/Attestations/GitHubActionsWorkflow@v1"
	PayloadContentType   = "application/vnd.in-toto+json"
)

var (
	artifactPath  = flag.String("artifact_path", "", "The file or dir path of the artifacts for which provenance should be generated.")
	outputPath    = flag.String("output_path", "build.provenance", "The path to which the generated provenance should be written.")
	githubContext = flag.String("github_context", "", "The '${github}' context value.")
	runnerContext = flag.String("runner_context", "", "The '${runner}' context value.")
)

type Envelope struct {
	PayloadType string        `json:"payloadType"`
	Payload     string        `json:"payload"`
	Signatures  []interface{} `json:"signatures"`
}
type Statement struct {
	Type          string    `json:"_type"`
	Subject       []Subject `json:"subject"`
	PredicateType string    `json:"predicateType"`
	Predicate     `json:"predicate"`
}
type Subject struct {
	Name   string    `json:"name"`
	Digest DigestSet `json:"digest"`
}
type Predicate struct {
	Builder   `json:"builder"`
	Metadata  `json:"metadata"`
	Recipe    `json:"recipe"`
	Materials []Item `json:"materials"`
}
type Builder struct {
	Id string `json:"id"`
}
type Metadata struct {
	BuildInvocationId string `json:"buildInvocationId"`
	Completeness      `json:"completeness"`
	Reproducible      bool `json:"reproducible"`
	// BuildStartedOn not defined as it's not available from a GitHub Action.
	BuildFinishedOn string `json:"buildFinishedOn"`
}
type Recipe struct {
	Type              string          `json:"type"`
	DefinedInMaterial int             `json:"definedInMaterial"`
	EntryPoint        string          `json:"entryPoint"`
	Arguments         json.RawMessage `json:"arguments"`
	Environment       *AnyContext     `json:"environment"`
}
type Completeness struct {
	Arguments   bool `json:"arguments"`
	Environment bool `json:"environment"`
	Materials   bool `json:"materials"`
}
type DigestSet map[string]string
type Item struct {
	URI    string    `json:"uri"`
	Digest DigestSet `json:"digest"`
}

type AnyContext struct {
	GitHubContext `json:"github"`
	RunnerContext `json:"runner"`
}
type GitHubContext struct {
	Action          string          `json:"action"`
	ActionPath      string          `json:"action_path"`
	Actor           string          `json:"actor"`
	BaseRef         string          `json:"base_ref"`
	Event           json.RawMessage `json:"event"`
	EventName       string          `json:"event_name"`
	EventPath       string          `json:"event_path"`
	HeadRef         string          `json:"head_ref"`
	Job             string          `json:"job"`
	Ref             string          `json:"ref"`
	Repository      string          `json:"repository"`
	RepositoryOwner string          `json:"repository_owner"`
	RunId           string          `json:"run_id"`
	RunNumber       string          `json:"run_number"`
	SHA             string          `json:"sha"`
	Token           string          `json:"token,omitempty"`
	Workflow        string          `json:"workflow"`
	Workspace       string          `json:"workspace"`
}
type RunnerContext struct {
	OS        string `json:"os"`
	Temp      string `json:"temp"`
	ToolCache string `json:"tool_cache"`
}

// See https://docs.github.com/en/actions/reference/events-that-trigger-workflows
// The only Event with dynamically-provided input is workflow_dispatch which
// exposes the user params at the key "input."
type AnyEvent struct {
	Inputs json.RawMessage `json:"inputs"`
}

// subjects walks the file or directory at "root" and hashes all files.
func subjects(root string) ([]Subject, error) {
	// Check for broken symlinks along the root path.
	parents := []string{root}
	for {
		dir, _ := filepath.Split(parents[0])
		if dir == "" {
			break
		}
		parents = append([]string{filepath.Clean(dir)}, parents...)
	}
	for _, parent := range parents {
		if _, err := os.Stat(parent); err != nil {
			if path, _ := os.Readlink(parent); path != "" && os.IsNotExist(err) {
				return nil, errors.New("stat " + parent + ": broken symlink")
			} else {
				return nil, err
			}
		}
	}
	// Walk root path for subjects.
	var s []Subject
	return s, filepath.Walk(root, func(abspath string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relpath, err := filepath.Rel(root, abspath)
		if err != nil {
			return err
		}
		// Note: filepath.Rel() returns "." when "root" and "abspath" point to the same file.
		if relpath == "." {
			relpath = filepath.Base(root)
		}
		contents, err := ioutil.ReadFile(abspath)
		if err != nil {
			return err
		}
		sha := sha256.Sum256(contents)
		shaHex := hex.EncodeToString(sha[:])
		s = append(s, Subject{Name: relpath, Digest: DigestSet{"sha256": shaHex}})
		return nil
	})
}

func parseFlags() {
	flag.Parse()
	if *artifactPath == "" {
		fmt.Println("No value found for required flag: --artifact_path\n")
		flag.Usage()
		os.Exit(1)
	}
	if *outputPath == "" {
		fmt.Println("No value found for required flag: --output_path\n")
		flag.Usage()
		os.Exit(1)
	}
	if *githubContext == "" {
		fmt.Println("No value found for required flag: --github_context\n")
		flag.Usage()
		os.Exit(1)
	}
	if *runnerContext == "" {
		fmt.Println("No value found for required flag: --runner_context\n")
		flag.Usage()
		os.Exit(1)
	}
}

func main() {
	parseFlags()
	stmt := Statement{PredicateType: "https://in-toto.io/Provenance/v0.1", Type: "https://in-toto.io/Statement/v0.1"}
	subjects, err := subjects(*artifactPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	stmt.Subject = append(stmt.Subject, subjects...)
	stmt.Predicate = Predicate{
		Builder{},
		Metadata{
			Completeness: Completeness{
				Arguments:   true,
				Environment: false,
				Materials:   false,
			},
			Reproducible:    false,
			BuildFinishedOn: time.Now().UTC().Format(time.RFC3339),
		},
		Recipe{
			Type:              TypeId,
			DefinedInMaterial: 0,
		},
		[]Item{},
	}

	context := AnyContext{}
	if err := json.Unmarshal([]byte(*githubContext), &context.GitHubContext); err != nil {
		panic(err)
	}
	if err := json.Unmarshal([]byte(*runnerContext), &context.RunnerContext); err != nil {
		panic(err)
	}
	gh := context.GitHubContext
	// Remove access token from the generated provenance.
	context.GitHubContext.Token = ""
	// NOTE: Re-runs are not uniquely identified and can cause run ID collisions.
	repoURI := "https://github.com/" + gh.Repository
	stmt.Predicate.Metadata.BuildInvocationId = repoURI + "/actions/runs/" + gh.RunId
	// NOTE: This is inexact as multiple workflows in a repo can have the same name.
	// See https://github.com/github/feedback/discussions/4188
	stmt.Predicate.Recipe.EntryPoint = gh.Workflow
	event := AnyEvent{}
	if err := json.Unmarshal(context.GitHubContext.Event, &event); err != nil {
		panic(err)
	}
	stmt.Predicate.Recipe.Arguments = event.Inputs
	stmt.Predicate.Materials = append(stmt.Predicate.Materials, Item{URI: "git+" + repoURI, Digest: DigestSet{"sha1": gh.SHA}})
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		stmt.Predicate.Builder.Id = repoURI + GitHubHostedIdSuffix
	} else {
		stmt.Predicate.Builder.Id = repoURI + SelfHostedIdSuffix
	}

	// NOTE: At L1, writing the in-toto Statement type is sufficient but, at
	// higher SLSA levels, the Statement must be encoded and wrapped in an
	// Envelope to support attaching signatures.
	payload, _ := json.MarshalIndent(stmt, "", "  ")
	fmt.Println("Provenance:\n" + string(payload))
	if err := ioutil.WriteFile(*outputPath, payload, 0755); err != nil {
		fmt.Println("Failed to write provenance: %s", err)
		os.Exit(1)
	}
}
