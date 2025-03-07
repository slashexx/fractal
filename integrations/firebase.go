package integrations

import (
	"context"
	"encoding/json"
	_ "errors"
	"fmt"
	_ "strings"
	"sync"
	"time"

	firebase "firebase.google.com/go"
	_ "google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"github.com/SkySingh04/fractal/interfaces"
	"github.com/SkySingh04/fractal/logger"
	"github.com/SkySingh04/fractal/registry"
)

type FirebaseSource struct {
	CredentialFileAddr string `json:"firebase_credential_file"`
	Collection         string `json:"firebase_collection"`
	Document           string `json:"firebase_document"`
}

type FirebaseDestination struct {
	CredentialFileAddr string `json:"firebase_credential_file"`
	Collection         string `json:"firebase_collection"`
	Document           string `json:"firebase_document"`
}

func (f FirebaseSource) FetchData(req interfaces.Request) (interface{}, error) {
	logger.Infof("Connecting to Firebase Source: Collection=%s, using Service Account=%s", req.Collection, req.CredentialFileAddr)

	opt := option.WithCredentialsFile(req.CredentialFileAddr)
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Firebase app: %w", err)
	}

	client, err := app.Firestore(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Firestore client: %w", err)
	}
	defer client.Close()

	docs, err := client.Collection(req.Collection).Documents(context.Background()).GetAll()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch documents: %w", err)
	}

	logger.Infof("Fetched documents from Firebase: %d documents", len(docs))
	for i, doc := range docs {
		logger.Infof("Document %d ID: %s, Data: %v", i, doc.Ref.ID, doc.Data())
	}

	var allData []map[string]interface{}
	for _, doc := range docs {
		data := doc.Data() 
		logger.Infof("Fetched data from Firebase: %v", data)

		data["_id"] = doc.Ref.ID

		validatedData := data            
		transformedData := validatedData 

		allData = append(allData, transformedData)
	}

	return allData, nil
}

func (f FirebaseDestination) SendData(data interface{}, req interfaces.Request) error {
	logger.Infof("Writing data to Firebase database: Collection=%s, Document=%s", req.Collection, req.Document)

	opt := option.WithCredentialsFile(req.CredentialFileAddr)
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		return fmt.Errorf("failed to initialize Firebase app: %w", err)
	}

	client, err := app.Firestore(context.Background())
	if err != nil {
		return fmt.Errorf("failed to initialize Firestore client: %w", err)
	}
	defer client.Close()

	var post map[string]interface{}
	if err := convertToMap(data, &post); err != nil {
		return fmt.Errorf("failed to convert data: %w", err)
	}

	errChan := make(chan error, 1)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := client.Collection(req.Collection).NewDoc().Create(context.Background(), post)
		if err != nil {
			errChan <- fmt.Errorf("error writing to Firestore: %w", err)
		}
	}()

	wg.Wait()
	close(errChan)

	select {
	case err := <-errChan:
		return err
	default:
		logger.Infof("Successfully written data to Firestore: Collection=%s, Document=%s", req.Collection, req.Document)
		return nil
	}
}

func convertToMap(data interface{}, result *map[string]interface{}) error {
	logger.Infof("Firebase data to map: %v", data)

	temp, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data to JSON: %w", err)
	}

	if err := json.Unmarshal(temp, result); err != nil {
		return fmt.Errorf("failed to unmarshal JSON to map: %w", err)
	}
	return nil
}

func validateFirebaseData(data map[string]interface{}) (map[string]interface{}, error) {
	logger.Infof("Validating Firebase data: %v", data)
	// // message, ok := data;
	// if !ok || strings.TrimSpace(message) == "" {
	// 	return nil, errors.New("invalid or missing 'data' field")
	// }
	return data, nil
}

func transformFirebaseData(data map[string]interface{}) map[string]interface{} {
	logger.Infof("Transforming Firebase data: %v", data)
	// if message, ok := data["data"].(string); ok {
	// 	data["data"] = strings.ToUpper(message)
	// }
	data["processed"] = time.Now().Format(time.RFC3339)
	return data
}

func init() {
	registry.RegisterSource("Firebase", FirebaseSource{})
	registry.RegisterDestination("Firebase", FirebaseDestination{})
}
