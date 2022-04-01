package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	version "github.com/hashicorp/go-version"
	yaml "gopkg.in/yaml.v2"
)

type Chart struct {
	Name       string `yaml:"name"`
	Repo       string `yaml:"repo"`
	Url        string `yaml:"url"`
	Version    string `yaml:"version"`
	OldVersion string `yaml:"-"`
}

type Repo struct {
	Version string `yaml:"version"`
}

func main() {

	workingDir := os.Getenv("WORKING_DIRECTORY")
	chartFile := os.Getenv("CHART_FILE")
	noWrite := os.Getenv("NO_WRITE")

	if len(workingDir) == 0 {
		workingDir = "/github/workspace"
	}

	if len(chartFile) == 0 {
		chartFile = "charts.yaml"
	}

	chartPath := workingDir + "/" + chartFile

	log.Printf("Opening %s file from %s...", chartFile, chartPath)
	chartYaml, err := os.ReadFile(chartPath)

	Check(err)

	MultilineLog(string(chartYaml))

	updates := false
	var updatedCharts []Chart

	var chartsTmp []Chart

	yaml.UnmarshalStrict(chartYaml, &chartsTmp)

	charts := chartsTmp

	for i, chart := range charts {

		log.Printf("Getting %s Helm repository from %s", chart.Name, chart.Url)

		helmCommands := []string{
			fmt.Sprintf("helm repo add %s %s", chart.Repo, chart.Url),
			fmt.Sprintf("helm repo update %s", chart.Repo),
		}

		for _, cmd := range helmCommands {
			Command(cmd, "")
		}

		log.Printf("Pulling %s versions", chart.Name)

		versionYaml := Command(fmt.Sprintf("helm search repo %s/%s -l -o yaml", chart.Repo, chart.Name), "")

		var repo []Repo

		yaml.UnmarshalStrict(versionYaml, &repo)

		currentVersion, _ := version.NewVersion(chart.Version)
		latestVersion, _ := version.NewVersion(repo[0].Version)

		if currentVersion.LessThan(latestVersion) {
			log.Printf("Found newer version: %s", latestVersion)
			charts[i].OldVersion = currentVersion.Original()
			charts[i].Version = latestVersion.Original()
			updatedCharts = append(updatedCharts, charts[i])
			updates = true
		} else {
			log.Printf("Current version %s latest", currentVersion)
		}

	}

	if !updates {
		log.Print("No newer versions found, nothing to do.")
		os.Exit(0)
	}

	if len(noWrite) > 0 {
		log.Print("NO_WRITE environmental variable set, preventing file writing and pull request")
		os.Exit(0)
	}

	log.Print("Newer versions found, updating charts.yaml")

	newChartsYaml, err := yaml.Marshal(charts)
	Check(err)

	err = os.WriteFile(chartPath, newChartsYaml, 0666)
	Check(err)

	log.Print("Written new chart version configuration to charts.yaml:")
	MultilineLog(string(newChartsYaml))

	log.Print("Creating pull request...")
	PullRequest(updatedCharts, workingDir)

}

func Command(inputString string, dir string) []byte {

	log.Print(fmt.Sprintf("$ %s", inputString))
	input := strings.Split(inputString, " ")

	cmd := exec.Command(
		input[0], input[1:]...,
	)

	if dir != "" || len(dir) > 0 {
		cmd.Dir = dir
	}

	outputBytes, err := cmd.Output()

	Check(err)
	MultilineLog(string(outputBytes))

	return outputBytes
}

func Check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func MultilineLog(input string) {

	lines := strings.Split(input, "\n")

	for _, line := range lines {
		log.Print(line)
	}
}

func PullRequest(charts []Chart, workingDir string) {

	branchName := "tf-helm-update"
	pullRequestTitle := "Bump "
	pullRequestBody := ""

	for _, chart := range charts {
		pullRequestTitle += fmt.Sprintf(
			"%s from %s to %s, ",
			chart.Name,
			chart.OldVersion,
			chart.Version,
		)

		pullRequestBody += fmt.Sprintf(
			"## Terraform Helm Updater\nBumps %s Helm Chart version from %s to %s.\n",
			chart.Name,
			chart.OldVersion,
			chart.Version,
		)
	}

	pullRequestTitle = pullRequestTitle[:len(pullRequestTitle)-2]

	log.Print("Pull Request Title:")
	log.Print(pullRequestTitle)
	log.Print("Pull Request Body:")
	MultilineLog(pullRequestBody)

	noPR := os.Getenv("NO_PR")

	if len(noPR) > 0 {
		log.Print("NO_PR environmental variable set, preventing pull request")
		os.Exit(0)
	}

	log.Print(fmt.Sprintf("Creating new branch %s...", branchName))
	Command(fmt.Sprintf("git checkout -b %s", branchName), workingDir)
	log.Print("Branch sucessfully created!")

	log.Print("Committing changes to remote branch...")
	Command("git add -A", workingDir)
	Command("git commit -m \"Updated chart versions\"", workingDir)
	Command(fmt.Sprintf("git push -u origin %s", branchName), workingDir)
	log.Print("Successfully pushed changes to remote branch!")

	mainBranch := os.Getenv("MAIN_BRANCH")

	if len(mainBranch) == 0 {
		mainBranch = "main"
	}

	log.Print("Creating pull request...")
	Command(fmt.Sprintf(
		"gh pr create -t %s -b %s -B %s -H %s -l \"dependencies\" -l \"github_actions\"",
		pullRequestTitle,
		pullRequestBody,
		mainBranch,
		branchName,
	), workingDir)
	log.Print("Successfully created pull request!")

}
