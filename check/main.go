package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"time"

	auth "golang.org/x/oauth2/google"
)

type JSONSource struct {
	Source struct {
		Project    string `json:"project"`
		Instance   string `json:"instance"`
		PrivateKey string `json:"private_key"`
	} `json:"source"`
	Version struct {
		BackupID string `json:"backup_id"`
	} `json:"version"`
}
type BackupItem struct {
	Kind            string    `json:"kind"`
	Status          string    `json:"status"`
	EnqueuedTime    time.Time `json:"enqueuedTime"`
	BackupID        string    `json:"id"`
	StartTime       time.Time `json:"startTime"`
	EndTime         time.Time `json:"endTime"`
	Type            string    `json:"type"`
	WindowStartTime time.Time `json:"windowStartTime"`
	Instance        string    `json:"instance"`
	SelfLink        string    `json:"selfLink"`
	Location        string    `json:"location"`
	BackupKind      string    `json:"backupKind"`
}
type BackupRunsList struct {
	Kind  string       `json:"kind"`
	Items []BackupItem `json:"items"`
}
type BackupID struct {
	BackupID string `json:"backup_id"`
}

func WriteCredentialToFile(key string) error {
	file, err := os.Create("/service-account.json")
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(key)
	if err != nil {
		return err
	}
	os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/service-account.json")
	return nil
}

func getAuthToken() string {
	ctx := context.Background()
	scopes := []string{
		"https://www.googleapis.com/auth/cloud-platform",
	}
	credentials, err := auth.FindDefaultCredentials(ctx, scopes...)
	if err != nil {
		log.Fatal(err)
	}
	token, err := credentials.TokenSource.Token()
	if err != nil {
		log.Fatal(err)
	}

	return (fmt.Sprintf("Bearer %v", string(token.AccessToken)))
}
func ListBackupRuns(input JSONSource) ([]BackupItem, error) {
	url := fmt.Sprintf("https://sqladmin.googleapis.com/v1/projects/%s/instances/%s/backupRuns", input.Source.Project, input.Source.Instance)
	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return []BackupItem{}, err
	}

	req.Header.Add("Authorization", getAuthToken())

	res, err := client.Do(req)
	if err != nil {
		return []BackupItem{}, err
	}
	defer res.Body.Close()

	var backupRunsList BackupRunsList
	err = json.NewDecoder(res.Body).Decode(&backupRunsList)
	if err != nil {
		return []BackupItem{}, err
	}

	// Sort operations based on InsertTime in descending order
	sort.Slice(backupRunsList.Items, func(i, j int) bool {
		return backupRunsList.Items[i].EndTime.Before(backupRunsList.Items[j].EndTime)
	})

	return backupRunsList.Items, nil
}
func GetBackupItem(input JSONSource) ([]BackupItem, error) {
	url := fmt.Sprintf("https://sqladmin.googleapis.com/v1/projects/%s/instances/%s/backupRuns/%s", input.Source.Project, input.Source.Instance, input.Version.BackupID)
	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return []BackupItem{}, err
	}

	req.Header.Add("Authorization", getAuthToken())

	res, err := client.Do(req)
	if err != nil {
		return []BackupItem{}, err
	}
	defer res.Body.Close()

	var backupItem BackupItem
	err = json.NewDecoder(res.Body).Decode(&backupItem)
	if err != nil {
		return []BackupItem{}, err
	}

	// Create a slice of BackupItem and append the BackupItemRespond to it
	backupItems := []BackupItem{}
	backupItems = append(backupItems, backupItem)

	return backupItems, nil
}
func main() {
	var input JSONSource
	// Decode JSON from stdin into input struct
	decoder := json.NewDecoder(os.Stdin)
	err := decoder.Decode(&input)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	err = WriteCredentialToFile(string(input.Source.PrivateKey))
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	var backupIDs []BackupItem
	if input.Version.BackupID == "" {
		backupIDs, err = ListBackupRuns(input)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		backupIDs, err = GetBackupItem(input)
		if err != nil {
			log.Fatal(err)
		}
		backupIDs, err = ListBackupRuns(input)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Create a slice of BackupID structs and populate it with the backup IDs
	backupIDList := make([]BackupID, len(backupIDs))
	for i, backupID := range backupIDs {
		backupIDList[i] = BackupID{backupID.BackupID}
	}

	// Encode the slice of BackupID structs to JSON and print it
	output, err := json.MarshalIndent(backupIDList, "", "  ")
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	fmt.Println(string(output))
}
