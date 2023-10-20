package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
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
type Version struct {
	ID string `json:"backup_id"`
}
type Metadata struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type Output struct {
	Version  Version    `json:"version"`
	Metadata []Metadata `json:"metadata"`
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
func writeOutputToFile(output BackupItem, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encodedOutput, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}

	_, err = file.Write(encodedOutput)
	if err != nil {
		return err
	}

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

func GetBackupState(input JSONSource) (BackupItem, error) {
	url := fmt.Sprintf("https://sqladmin.googleapis.com/v1/projects/%s/instances/%s/backupRuns/%s", input.Source.Project, input.Source.Instance, input.Version.BackupID)
	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return BackupItem{}, err
	}

	req.Header.Add("Authorization", getAuthToken())

	res, err := client.Do(req)
	if err != nil {
		return BackupItem{}, err
	}
	defer res.Body.Close()

	var backupRun BackupItem
	err = json.NewDecoder(res.Body).Decode(&backupRun)
	if err != nil {
		return BackupItem{}, err
	}
	return backupRun, nil

}

func main() {
	log.SetFlags(0)
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

	outputPath := fmt.Sprintf("%s/output.json", os.Args[1])

	for {
		backupRun, err := GetBackupState(input)
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}
		if backupRun.Status == "SUCCESSFUL" {
			log.Println("Backup successful!")
			// Convert InsertTime to Taiwan time zone
			taiwanTimeZone, err := time.LoadLocation("Asia/Taipei")
			if err != nil {
				log.Fatal(err)
				os.Exit(1)
			}
			backupRun.EndTime = backupRun.EndTime.In(taiwanTimeZone)

			output := Output{
				Version: Version{ID: backupRun.BackupID},
				Metadata: []Metadata{
					{Name: "kind", Value: backupRun.BackupKind},
					{Name: "status", Value: backupRun.Status},
					{Name: "end-time", Value: backupRun.EndTime.Format(time.RFC3339)},
					{Name: "instance", Value: backupRun.Instance},
				},
			}
			err = writeOutputToFile(backupRun, outputPath)
			if err != nil {
				log.Fatal(err)
				os.Exit(1)
			}
			// print output as JSON
			encodedOutput, err := json.MarshalIndent(output, "", "  ")
			if err != nil {
				log.Fatal(err)
				os.Exit(1)
			}
			fmt.Println(string(encodedOutput))
			break
		} else if backupRun.Status == "ENQUEUED" || backupRun.Status == "RUNNING" || backupRun.Status == "PENDING" {
			log.Println("Backup state:", backupRun.Status)
			time.Sleep(30 * time.Second) // Wait 30 seconds before checking again
		} else {
			log.Panicln("Backup state:", backupRun.Status)
			os.Exit(1)
		}
	}
}
