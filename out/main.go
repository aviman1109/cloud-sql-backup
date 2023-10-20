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
	Parameters struct {
	} `json:"params"`
}
type InsertResult struct {
	Kind          string    `json:"kind"`
	TargetLink    string    `json:"targetLink"`
	Status        string    `json:"status"`
	User          string    `json:"user"`
	InsertTime    time.Time `json:"insertTime"`
	OperationType string    `json:"operationType"`
	OperationID   string    `json:"name"`
	TargetID      string    `json:"targetId"`
	SelfLink      string    `json:"selfLink"`
	TargetProject string    `json:"targetProject"`
	BackupContext struct {
		BackupID string `json:"backupId"`
		Kind     string `json:"kind"`
	} `json:"backupContext"`
}
type Version struct {
	BackupID string `json:"backup_id"`
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

func InsertBackupRuns(input JSONSource) (InsertResult, error) {
	url := fmt.Sprintf("https://sqladmin.googleapis.com/v1/projects/%s/instances/%s/backupRuns", input.Source.Project, input.Source.Instance)
	method := "POST"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return InsertResult{}, err
	}

	req.Header.Add("Authorization", getAuthToken())

	res, err := client.Do(req)
	if err != nil {
		return InsertResult{}, err
	}
	defer res.Body.Close()

	var backupAction InsertResult
	err = json.NewDecoder(res.Body).Decode(&backupAction)
	if err != nil {
		return InsertResult{}, err
	}
	return backupAction, nil

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

	backupAction, err := InsertBackupRuns(input)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	// Convert InsertTime to Taiwan time zone
	taiwanTimeZone, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	backupAction.InsertTime = backupAction.InsertTime.In(taiwanTimeZone)

	output := Output{
		Version: Version{BackupID: backupAction.BackupContext.BackupID},
		Metadata: []Metadata{
			{Name: "status", Value: backupAction.Status},
			{Name: "insert-time", Value: backupAction.InsertTime.Format(time.RFC3339)},
			{Name: "operation-id", Value: backupAction.OperationID},
			{Name: "operation-type", Value: backupAction.OperationType},
			{Name: "target-instance", Value: backupAction.TargetID},
		},
	}

	// print output as JSON
	encodedOutput, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	fmt.Println(string(encodedOutput))
}
