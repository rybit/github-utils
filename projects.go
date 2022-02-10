package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

type projectCard struct {
	ID         int
	Note       string `json:",omitempty"`
	ContentURL string `json:"content_url,omitempty"`
}

type projectColumn struct {
	ID   int
	Name string

	Cards []projectCard `json:",omitempty"`
}

type project struct {
	ID      int
	Name    string
	Body    string
	State   string
	HTMLURL string `json:"html_url"`

	Cols []*projectColumn `json:",omitempty"`
}

func projectCmd() *cobra.Command {
	cmd := cobra.Command{
		Use: "projects",
	}

	cmd.AddCommand(
		listProjectsCmd(),
		queryProjectCmd(),
		emptyProjectCmd(),
		migrateProjectCmd(),
	)

	// cmd.AddCommand(projectQLCommand())

	return &cmd
}

func listProjectsCmd() *cobra.Command {
	var projectID, org string
	cmd := cobra.Command{
		Use: "list",
		Run: func(cmd *cobra.Command, args []string) {
			readProjectPages(org, func(p project) error {
				return enc(&p)
			})
		},
	}
	cmd.Flags().StringVar(&projectID, "project-id", "", "a specific projectID to gather")
	cmd.PersistentFlags().StringVar(&org, "org", "netlify", "the organization to create the new project in")

	return &cmd
}

func queryProjectCmd() *cobra.Command {
	var shallow bool
	cmd := cobra.Command{
		Use: "query [id]",
		Run: func(cmd *cobra.Command, args []string) {
			enc(queryProject(args[0], shallow))
		},
		Args: cobra.ExactArgs(1),
	}
	cmd.Flags().BoolVar(&shallow, "shallow", false, "skip fetching the columns")
	return &cmd
}

func emptyProjectCmd() *cobra.Command {
	cmd := cobra.Command{
		Use: "clear [id]",
		Run: func(cmd *cobra.Command, args []string) {
			boardID := args[0]
			proj := queryProject(boardID, false)
			for _, c := range proj.Cols {
				log.Debug("removing column", zap.String("name", c.Name), zap.Int("id", c.ID))
				code, raw := queryGitHub(fmt.Sprintf("/projects/columns/%d", c.ID),
					withMethod(http.MethodDelete),
				)
				requireCode(204, code, raw)
			}
		},
		Args: cobra.ExactArgs(1),
	}
	return &cmd
}

func migrateProjectCmd() *cobra.Command {
	var useDisk bool
	var destProjectID string
	var org string
	cmd := cobra.Command{
		Use: "migrate [id]",
		Run: func(cmd *cobra.Command, args []string) {
			migrateProject(args[0], destProjectID, org, useDisk)
		},
		Args: cobra.ExactArgs(1),
	}
	cmd.Flags().StringVar(&destProjectID, "dest", "", "a specific projectID to push results to")
	cmd.Flags().StringVar(&org, "org", "netlify", "the organization to create the new project in")
	cmd.Flags().BoolVar(&useDisk, "disk", false, "if the provided project ID is a path to a json file")

	return &cmd
}

func migrateProject(inRef, destRef, org string, useDisk bool) {
	var proj project
	if useDisk {
		bs, err := ioutil.ReadFile(inRef)
		panicOnErr(err)
		panicOnErr(json.Unmarshal(bs, &proj))
		log.Debug("loaded project definition from disk")
	} else {
		proj = *queryProject(inRef, false)
	}

	if destRef == "" {
		destRef = createNewProject(proj, org)
		log.Debug("created new project board", zap.String("id", destRef))
	}

	var cardsCreated int
	for _, col := range proj.Cols {
		newColID := createColumn(destRef, col)
		log.Debug("created new column", zap.String("id", newColID))
		for _, card := range col.Cards {
			newCardID := createCard(newColID, card)
			log.Debug("created new card", zap.String("id", newCardID))
			cardsCreated++
		}
	}

	log.Info("migrated the board, columns, and cards", zap.Int("cards_created", cardsCreated))
}

func queryProject(id string, shallow bool) *project {
	code, raw := queryGitHub(fmt.Sprintf("/projects/%s", id))
	if code != http.StatusOK {
		log.Info("Got a !200 response, assuming we got all the pages")
		return nil
	}
	var proj project
	panicOnErr(json.Unmarshal(raw, &proj))

	if !shallow {
		proj.Cols = fetchCols(proj.ID)
	}
	return &proj
}

func readProjectPages(org string, iter func(p project) error) {
	log.Debug("going to list the project page by page", zap.String("org", org))
	projectsProcessed := 0
	path := fmt.Sprintf("/orgs/%s/projects", org)
	queryByPage(path, func(raw []byte) bool {
		objs := []project{}
		panicOnErr(json.Unmarshal(raw, &objs))
		log.Debug("parsed out new projects", zap.Int("count", len(objs)), zap.String("path", path))
		for _, r := range objs {
			if skipArchive && r.State != "open" {
				log.Debug("skipping closed project",
					zap.String("project", r.Name),
				)
				continue
			}

			log.Debug("starting to process project",
				zap.String("project", r.Name),
			)
			panicOnErr(iter(r))
			projectsProcessed++
			if limit != 0 && projectsProcessed >= limit {
				log.Debug("Reached configured limit")
				return false
			}
		}
		return len(objs) != 0
	})
}

func fetchCols(projID int) []*projectColumn {
	path := fmt.Sprintf("/projects/%d/columns", projID)
	code, raw := queryGitHub(path)
	if code != http.StatusOK {
		panic(fmt.Sprintf("got a %d in response to the query for cols: %s", code, path))
	}

	cols := []*projectColumn{}
	panicOnErr(json.Unmarshal(raw, &cols))
	for _, col := range cols {
		path = fmt.Sprintf("/projects/columns/%d/cards", col.ID)
		queryByPage(path, func(raw []byte) bool {
			cards := []projectCard{}
			panicOnErr(json.Unmarshal(raw, &cards))
			col.Cards = append(col.Cards, cards...)
			return len(cards) != 0
		})
		log.Debug("loaded cards",
			zap.Int("cards", len(col.Cards)),
			zap.String("path", path),
			zap.Int("project_id", projID),
			zap.Int("column_id", col.ID),
			zap.String("column_name", col.Name),
		)
	}

	return cols
}

func createNewProject(original project, org string) string {
	body, err := json.Marshal(&struct {
		Name string `json:"name"`
		Body string `json:"body"`
	}{
		Name: original.Name,
		Body: original.Body,
	})
	panicOnErr(err)

	code, raw := queryGitHub(fmt.Sprintf("/orgs/%s/projects", org),
		withMethod(http.MethodPost),
		withPayload(body),
	)
	requireCode(201, code, raw)
	var out project
	panicOnErr(json.Unmarshal(raw, &out))
	log.Info("created new project", zap.Int("id", out.ID), zap.String("url", out.HTMLURL))
	return strconv.Itoa(out.ID)
}

func createColumn(projectID string, col *projectColumn) string {
	body, err := json.Marshal(&struct {
		Name string `json:"name"`
	}{
		Name: col.Name,
	})
	panicOnErr(err)

	code, raw := queryGitHub(fmt.Sprintf("/projects/%s/columns", projectID),
		withMethod(http.MethodPost),
		withPayload(body),
	)
	requireCode(201, code, raw)
	var out projectColumn
	panicOnErr(json.Unmarshal(raw, &out))

	return strconv.Itoa(out.ID)
}

func createCard(columnID string, card projectCard) string {
	var payload interface{}
	if card.Note != "" {
		payload = struct {
			Note string `json:"note"`
		}{
			card.Note,
		}
	} else {
		// need to resolve the actual ID of the card
		code, raw := queryGitHub(card.ContentURL)
		requireCode(http.StatusOK, code, raw)
		issue := struct {
			ID int
		}{}
		panicOnErr(json.Unmarshal(raw, &issue))

		contentIs := "Issue"
		if strings.Contains(card.ContentURL, "/pulls/") {
			contentIs = "PullRequest"
		}

		payload = struct {
			ContentID   int    `json:"content_id"`
			ContentType string `json:"content_type"`
		}{
			ContentID:   issue.ID,
			ContentType: contentIs,
		}
	}

	body, err := json.Marshal(&payload)
	panicOnErr(err)
	code, raw := queryGitHub(fmt.Sprintf("/projects/columns/%s/cards", columnID),
		withMethod(http.MethodPost),
		withPayload(body),
	)
	requireCode(201, code, raw)
	var out projectColumn
	panicOnErr(json.Unmarshal(raw, &out))

	return strconv.Itoa(out.ID)
}
