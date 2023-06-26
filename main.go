package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"
)

type BuildChunk struct {
	Name string `json:"name"`
	Size int    `json:"size"`
	Path string `json:"path"`
	Url  string `json:"url"`
}

var ToDownloadSize int64
var DownloadedSize int64

func ConvertBytes(sizeInBytes float64) (float64, string) {

	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	if sizeInBytes >= GB {
		sizeInUnit := sizeInBytes / GB
		return sizeInUnit, "GB"
	} else if sizeInBytes >= MB {
		sizeInUnit := sizeInBytes / MB
		return sizeInUnit, "MB"
	} else if sizeInBytes >= KB {
		sizeInUnit := sizeInBytes / KB
		return sizeInUnit, "KB"
	}

	return sizeInBytes, "bytes"
}

func DownloadRoutine(mainDirPath string, buildChunk BuildChunk) {

	targetFilePath := path.Join(mainDirPath, buildChunk.Path)

	for i := 0; i < 5; i++ { // 5 max retries

		// Check if the file has already been downloaded
		fileInfo, err := os.Stat(targetFilePath)
		if err == nil {
			if fileInfo.Size() == int64(buildChunk.Size) {
				fmt.Printf("Already downloaded %s\n", buildChunk.Name)
				DownloadedSize += int64(buildChunk.Size)
				break
			}
		}

		// Create the directory if necessary
		err = os.MkdirAll(filepath.Dir(targetFilePath), os.ModePerm)
		if err != nil {
			fmt.Printf("Creating directory error handled, retrying: %s\n", buildChunk.Name)
			continue
		}

		// Create the file
		out, err := os.Create(targetFilePath)
		if err != nil {
			fmt.Printf("Creating file error handled, retrying: %s\n", buildChunk.Name)
			time.Sleep(time.Second)
			continue
		}
		defer out.Close()

		// Get the data
		resp, err := http.Get(buildChunk.Url)
		if err != nil {
			fmt.Printf("Server connection error handled, retrying: %s\n", buildChunk.Name)
			time.Sleep(time.Second)
			continue
		}
		defer resp.Body.Close()

		// Check server response
		if resp.StatusCode != http.StatusOK {
			fmt.Printf("Server response error handled, retrying: %s\n", buildChunk.Name)
			time.Sleep(time.Second)
			continue
		}

		// Write the body to file
		_, err = io.Copy(out, resp.Body)
		if err != nil {
			fmt.Printf("Copying file error handled, retrying: %s\n", buildChunk.Name)
			time.Sleep(time.Second)
			continue
		}

		DownloadedSize += int64(buildChunk.Size)

		downloadedSize, downloadedUnit := ConvertBytes(float64(DownloadedSize))
		toDownloadSize, toDownloadUnit := ConvertBytes(float64(ToDownloadSize))

		fmt.Printf("Downloaded: %s, Total downloaded: %.2f %s out of %.2f %s\n", buildChunk.Name, downloadedSize, downloadedUnit, toDownloadSize, toDownloadUnit)

		break // Close the goroutine
	}
}

func main() {

	args := os.Args

	// The only argument will be the install path
	if len(args) != 2 || args[1] == "" {
		fmt.Println("Error reading the arguments")
		return
	}

	mainDirPath := args[1]

	res, err := http.Get("https://raw.githubusercontent.com/FModifications/FModDownloader/main/BuildsData/Windows/8.51.json")
	if err != nil {
		fmt.Println("Error communicating with the cloudstorage")
		return
	}

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println("Error reading the response body")
		os.Exit(1)
	}

	var BuildChunks []BuildChunk
	err = json.Unmarshal(resBody, &BuildChunks)
	if err != nil {
		fmt.Println("Error parsing the response body")
		os.Exit(1)
	}

	for _, buildChunk := range BuildChunks {
		ToDownloadSize += int64(buildChunk.Size)
	}

	toDownloadSize, toDownloadUnit := ConvertBytes(float64(ToDownloadSize))
	fmt.Printf("To download size: %.2f %s\n", toDownloadSize, toDownloadUnit)

	var wg sync.WaitGroup

	for _, buildChunk := range BuildChunks {

		wg.Add(1)

		go func(chunk BuildChunk) {
			defer wg.Done()
			DownloadRoutine(mainDirPath, chunk)
		}(buildChunk)

	}

	wg.Wait() // Wait until every go routine ends

	fmt.Println("Succesfully downloaded!")

	select {}
}
