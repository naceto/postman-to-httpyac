package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// URL -
type URL struct {
	Raw  string   `json:"raw"`
	Host []string `json:"host"`
	Path []string `json:"path"`
}

// Header -
type Header struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Body -
type Body struct {
	Raw  string `json:"raw"`
	Mode string `json:"mode"`
}

// Request -
type Request struct {
	Method string          `json:"method"`
	URL    json.RawMessage `json:"url"`
	Header []*Header       `json:"header"`
	Body   json.RawMessage `json:"body"`
}

// Item -
type Item struct {
	Name    string   `json:"name"`
	Request *Request `json:"request"`
	Items   []*Item  `json:"item"`
}

// PostmanCollection -
type PostmanCollection struct {
	Items []*Item `json:"item"`
}

// EnvironmentItem -
type EnvironmentItem struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// PostmanEnvironment -
type PostmanEnvironment struct {
	Name   string             `json:"name"`
	Values []*EnvironmentItem `json:"values"`
}

func (e *PostmanEnvironment) String() string {
	sb := strings.Builder{}
	for _, v := range e.Values {
		sb.WriteString(fmt.Sprintf("%s=%s\n", v.Key, v.Value))
	}
	return sb.String()
}

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: postman-to-httpyac-converter <collections-dir> <environments-dir>")
		os.Exit(1)
	}

	collectionsDir := os.Args[1]
	environmentsDir := os.Args[2]

	// Read all collection files in the collections directory
	collectionFiles, err := os.ReadDir(collectionsDir)
	if err != nil {
		fmt.Printf("Error reading collections directory: %v\n", err)
		os.Exit(1)
	}

	// Read all environment files in the environments directory
	environmentFiles, err := os.ReadDir(environmentsDir)
	if err != nil {
		fmt.Printf("Error reading environments directory: %v\n", err)
		os.Exit(1)
	}

	// Create subdirectories for collections and environments
	collectionsSubdir := "parsed-collections"
	environmentsSubdir := "parsed-environments"
	err = os.MkdirAll(collectionsSubdir, os.ModePerm)
	if err != nil {
		fmt.Printf("Error creating collections subdirectory: %v\n", err)
		os.Exit(1)
	}
	err = os.MkdirAll(environmentsSubdir, os.ModePerm)
	if err != nil {
		fmt.Printf("Error creating environments subdirectory: %v\n", err)
		os.Exit(1)
	}

	// Process collections
	for _, fileInfo := range collectionFiles {
		if !fileInfo.IsDir() && strings.HasSuffix(fileInfo.Name(), ".json") {
			collectionFileName := filepath.Join(collectionsDir, fileInfo.Name())
			outputDir := filepath.Join(collectionsSubdir, strings.TrimSuffix(sanitizeName(fileInfo.Name()), ".postman_collection.json"))

			// Create subdirectory for the collection
			err := os.MkdirAll(outputDir, os.ModePerm)
			if err != nil {
				fmt.Printf("Error creating collection subdirectory: %v\n", err)
				continue
			}

			// Read the Postman Collection 2.1 JSON file
			collectionData, err := os.ReadFile(collectionFileName)
			if err != nil {
				fmt.Printf("Error reading collection file: %v\n", err)
				continue
			}

			// Parse the JSON data
			var collection PostmanCollection
			if err := json.Unmarshal(collectionData, &collection); err != nil {
				fmt.Printf("Error parsing collection %s JSON: %v\n", collectionFileName, err)
				continue
			}

			// Convert and save collection requests
			convertAndSaveCollection(collection.Items, outputDir)

			fmt.Printf("Converted collection: %s\n", fileInfo.Name())
		}
	}

	// Process environments
	for _, fileInfo := range environmentFiles {
		if !fileInfo.IsDir() && strings.HasSuffix(fileInfo.Name(), ".json") {
			environmentFileName := filepath.Join(environmentsDir, fileInfo.Name())

			// Read the environment JSON file
			environmentData, err := os.ReadFile(environmentFileName)
			if err != nil {
				fmt.Printf("Error reading environment file: %v\n", err)
				continue
			}

			// Parse the JSON data
			var environment PostmanEnvironment
			if err := json.Unmarshal(environmentData, &environment); err != nil {
				fmt.Printf("Error parsing environment %s JSON: %v\n", environmentFileName, err)
				continue
			}

			// Write the environment JSON data to a .env file
			envFileName := filepath.Join(environmentsSubdir, sanitizeName(environment.Name+".env"))
			err = os.WriteFile(envFileName, []byte(environment.String()), 0644)
			if err != nil {
				fmt.Printf("Error writing .env file for environment %s: %v\n", fileInfo.Name(), err)
			}

			fmt.Printf("Converted environment: %s\n", fileInfo.Name())
		}
	}
}

func convertAndSaveCollection(items []*Item, outputDir string) {
	// Iterate through each request in the collection and write it to a separate .http file
	for _, item := range items {
		// First level request in collection
		if item.Request != nil {
			// Create an HTTPYac request and add environment variables
			httpYacRequest, err := convertToHTTPYacRequest(item.Request)
			if err != nil {
				fmt.Printf("Error converting request to httpYac: %v\n", err)
				continue
			}

			// Write the HTTPYac request to a separate .http file
			requestFileName := filepath.Join(outputDir, sanitizeName(item.Name+".http"))
			err = ioutil.WriteFile(requestFileName, []byte(httpYacRequest), 0644)
			if err != nil {
				fmt.Printf("Error writing .http file for request %s: %v\n", item.Name, err)
			}
		}

		// Subfolder request in collection
		if len(item.Items) > 0 {
			nestedOutputDir := filepath.Join(outputDir, sanitizeName(item.Name))
			// Create subdirectory for the collection
			err := os.MkdirAll(nestedOutputDir, os.ModePerm)
			if err != nil {
				fmt.Printf("Error creating collection subdirectory: %v\n", err)
				continue
			}

			convertAndSaveCollection(item.Items, nestedOutputDir)
		}
	}
}

func sanitizeName(fileName string) string {
	sanitizedFileName := strings.Map(func(r rune) rune {
		switch r {
		case ':', '/', '\\', '?', '*', '<', '>', '|', '"':
			return '_'
		}
		return r
	}, fileName)
	return sanitizedFileName
}

func convertToHTTPYacRequest(request *Request) (string, error) {
	// Parse the URL
	var url URL
	if err := json.Unmarshal(request.URL, &url); err != nil {
		url.Raw = string(request.URL)
	}

	sb := strings.Builder{}

	sb.WriteString(fmt.Sprintf("%s %s\n", request.Method, url.Raw))
	for _, header := range request.Header {
		sb.WriteString(fmt.Sprintf("%s: %s\n", header.Key, header.Value))
	}

	sb.WriteString("\n")
	if request.Body != nil {
		var body Body
		if err := json.Unmarshal(request.Body, &body); err != nil {
			body.Raw = string(request.Body)
		}
		sb.WriteString(body.Raw)
	}

	httpYacRequest := sb.String()
	return httpYacRequest, nil
}
