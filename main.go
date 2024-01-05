package main

import (
	"bufio"
	"context"
	"fmt"
	"maps"
	"os"
	"strings"

	"github.com/google/go-github/v57/github"
)

const (
	org         = "openshift"
	ownersFile  = "OWNERS"
	ownersAlias = "OWNERS_ALIASES"
)

func main() {
	token := os.Getenv("Github_Token")
	if token == "" {
		fmt.Println("No github token found in ENV Github_Token")
		os.Exit(0)
	}

	gc := githubClient{github.NewClient(nil).WithAuthToken(token)}

	orgUsers, err := gc.getOrgUsers(org)
	if err != nil {
		fmt.Println("Error getting user list ", err.Error())
		os.Exit(0)
	}

	allRepos, err := gc.getOrgReposNames(org)
	if err != nil {
		fmt.Println("Could not get repo names", err.Error())
		os.Exit(0)
	}
	fmt.Println(len(allRepos))

	counter := 0
	for _, repo := range allRepos {
		// look at owners file for every repo
		ownerContent, err := gc.getFileContent(org, repo, ownersFile)
		if err != nil {
			// if the file is not found repo does not have and owners file so move on to the next one
			if strings.Contains(err.Error(), "404") {
				continue
			}
			fmt.Printf("Error Reading content for %s \n Error :: %s\n", repo, err.Error())
			continue
		}

		aliasesContent := ""
		aliasesContent, err = gc.getFileContent(org, repo, ownersAlias)
		if err != nil && strings.Contains(err.Error(), "404") {
			fmt.Printf("Error Reading alias file for %s \n Err :: %s\n", repo, err.Error())
		}
		aliases := map[string]bool{}
		aliasUsers := map[string]bool{}
		if aliasesContent != "" {
			aliasUsers = parseUsers(aliasesContent)
			aliases = parseAliases(aliasesContent)
		}

		ownerUsers := parseUsers(ownerContent)

		// remove any aliases from the ownerUsers map
		// then add any users from the alias list not already in the owners map
		ownerUsers = removeMapFromMap(aliases, ownerUsers)
		maps.Copy(ownerUsers, aliasUsers)
		invalidUsers := usersNotInOrg(orgUsers, ownerUsers)

		if len(invalidUsers) > 0 {
			counter++
			fmt.Println("----------------------------------")
			fmt.Println("Invalid Users for repo :: ", repo)
			fmt.Println(invalidUsers)
		}
	}
	fmt.Printf("\n******************************\n")
	fmt.Println("Total Repos with invalid users :: ", counter)
	fmt.Printf("\n******************************\n")
}

func removeMapFromMap(remove, keep map[string]bool) map[string]bool {
	for k := range remove {
		delete(keep, k)
	}
	return keep
}

func usersNotInOrg(orgUsers, ownerUsers map[string]bool) []string {
	var invalidUsers []string
	for k := range ownerUsers {
		if _, ok := orgUsers[k]; !ok {
			invalidUsers = append(invalidUsers, k)
		}
	}
	return invalidUsers
}

func parseAliases(content string) map[string]bool {
	aliases := map[string]bool{}
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasSuffix(line, ":") {
			if strings.Contains(line, "#") {
				line = strings.Split(line, "#")[0]
			}
		}
		alias := strings.TrimSpace(strings.TrimSuffix(line, ":"))
		if alias != "" {
			aliases[alias] = true
		}

	}
	return map[string]bool{}
}

func parseUsers(content string) map[string]bool {
	users := map[string]bool{}
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "-") {
			if strings.Contains(line, "#") {
				line = strings.Split(line, "#")[0]
			}

			name := strings.TrimSpace(strings.TrimPrefix(line, "-"))
			users[name] = true
		}
	}
	return users
}

type githubClient struct {
	Client *github.Client
}

func (gc githubClient) getOrgUsers(org string) (map[string]bool, error) {
	allUsers := map[string]bool{}
	opt := &github.ListMembersOptions{
		ListOptions: github.ListOptions{PerPage: 500},
	}
	for {
		users, resp, err := gc.Client.Organizations.ListMembers(context.Background(), org, opt)
		if err != nil {
			return map[string]bool{}, err
		}
		for _, user := range users {
			allUsers[user.GetLogin()] = true
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return allUsers, nil
}

func (gc githubClient) getOrgReposNames(org string) ([]string, error) {
	var allRepos []string
	opt := &github.RepositoryListByOrgOptions{
		Type:        "source",
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		repos, resp, err := gc.Client.Repositories.ListByOrg(context.Background(), org, opt)
		if err != nil {
			return []string{}, err
		}
		for _, repo := range repos {
			allRepos = append(allRepos, repo.GetName())
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return allRepos, nil
}

// THis function can return 404 if the file is not found so ignore that if you wish
func (gc githubClient) getFileContent(orgName, repoName, fileName string) (string, error) {
	// look at owners file
	file, _, _, err := gc.Client.Repositories.GetContents(
		context.Background(),
		orgName,
		repoName,
		fileName,
		nil,
	)
	if err != nil {
		return "", err
	}

	content, err := file.GetContent()
	if err != nil {
		return "", err
	}

	return content, nil
}
