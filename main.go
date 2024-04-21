package main

import (
	"os"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"bytes"
	"os/exec"
	"regexp"
	"path/filepath"
	"net/url"
	"encoding/json"
	"sort"
	"time"
)

// sample config.json file:
// {
//     "port": "8100",
//     "filesDir": "/home/user/mp3s",
// 	   "dirSizeMaxMB": 200,
//     "execDir": "/home/user/downloads/yt-dlp"
// }

type Config struct {
	Port     string `json:"port"`
	FilesDir string `json:"filesDir"`
	DirSizeMaxMB int `json:"dirSizeMaxMB"`
	ExecPath  string `json:"execDir"`
}

var filesDir string
var execPath string
var limitBytes int64

func main() {

	// parsing command line arguments and/or config default values --
	configFilePath := flag.String("c", "config.json", "path to json config file")
	flag.Parse()
	config, err := getConfig(*configFilePath)
	if err != nil {
		log.Fatal("Error getting config: %v\n", err)
	}
	port := flag.String("p", config.Port, "port to serve on")
	flag.StringVar(&filesDir, "d", config.FilesDir, "the directory where files are hosted")
	dirSizeMaxMB := flag.Int("m", config.DirSizeMaxMB, "maximum size of hosted directory")
	flag.StringVar(&execPath, "e", config.ExecPath, "path of the yt-dlp executable")
	flag.Parse()
	limitBytes = int64(*dirSizeMaxMB) * 1024 * 1024
	// ---

	// serve files from the path specified by filesDir arg or default
	http.Handle("/files/", http.StripPrefix("/files/", http.FileServer(http.Dir(filesDir))))

	// POST endpoint, expects body to be a string of a youtube url
	http.HandleFunc("/mp3", handleMp3)

	log.Printf("Serving %s on HTTP port: %s\n", filesDir, *port)
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}

func handleMp3(w http.ResponseWriter, r *http.Request) {
	// Handle preflight OPTIONS request
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading request body: %v", err), http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	// body is expected to be a url string
	url := string(body)
	if url == "" {
		http.Error(w, "URL not provided", http.StatusBadRequest)
		return
	}

	if !isValidURL(url) {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	// videoID is used as the mp3 filename
	videoID := getVideoID(url)
	if (videoID == "") {
		http.Error(w, "Bad YT URL", http.StatusBadRequest)
		return
	}

	if (fileAlreadyExists(videoID)) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, fmt.Sprintf("%s/%s/%s.mp3", r.Host, "files", videoID))
		return
	}

	// call the yt-dlp executable with audio only mp3 options
	outfile := filepath.Join(filesDir, fmt.Sprintf("%s.mp3", videoID))
	cmdArgs := []string{url, "-x", "--audio-format", "mp3", "-o", outfile}
	cmd := exec.Command(execPath, cmdArgs...)
	var out bytes.Buffer
    cmd.Stdout = &out
    err = cmd.Run()

    if err != nil {
        http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf(out.String())
		return
    }

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, fmt.Sprintf("%s/%s/%s.mp3", r.Host, "files", videoID))

	// manage hosting limit ---
	totalSize, err := getTotalSize(filesDir)
	if err != nil {
		log.Println("Error:", err)
		return
	}

	if totalSize > limitBytes {
		oldestMP3File, err := getOldestMP3File(filesDir)
		if err != nil {
			log.Println("Error:", err)
			return
		}

		log.Printf("Info: Folder size is %d bytes. Deleting: %s\n", totalSize, oldestMP3File)
		err = os.Remove(oldestMP3File)
		if err != nil {
			log.Println("Error deleting file:", err)
			return
		}
	}
}

func isValidURL(url string) bool {
	regex := regexp.MustCompile(`^(http|https)://[^ "]+$`)
	return regex.MatchString(url)
}

func getVideoID(inputURL string) string {
	urlPattern := regexp.MustCompile(`^https://youtu\.be/([a-zA-Z0-9_-]+)$`)
	match := urlPattern.FindStringSubmatch(inputURL)

	if len(match) > 1 {
		videoID := match[1]
		log.Printf("Video ID: %s", videoID)
		return videoID
	}

	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		log.Printf("Error parsing URL: %v", err)
		return ""
	}

	videoID := parsedURL.Query().Get("v")
	if videoID != "" {
		return videoID
	}

	return ""
}

func fileAlreadyExists(videoID string) bool {
	mp3Path := filepath.Join(filesDir, fmt.Sprintf("%s.mp3", videoID))

	if _, err := os.Stat(mp3Path); os.IsNotExist(err) {
		fmt.Printf("File %s does not exist.\n", mp3Path)
		return false
	} else if err != nil {
		fmt.Printf("Error checking file %s: %v\n", mp3Path, err)
		return false
	} else {
		fmt.Printf("File %s exists.\n", mp3Path)
		return true
	}
}


func getConfig(configFilePath string) (Config, error) {
	var config Config
	configFile, err := os.Open(configFilePath)
	if err != nil {
		return config, fmt.Errorf("error opening config file: %v", err)
	}
	defer configFile.Close()

	err = json.NewDecoder(configFile).Decode(&config)
	if err != nil {
		return config, fmt.Errorf("error decoding config file: %v", err)
	}

	return config, nil
}

// size of folder
func getTotalSize(folderPath string) (int64, error) {
	var totalSize int64

	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	if err != nil {
		return 0, err
	}

	return totalSize, nil
}

func getOldestMP3File(folderPath string) (string, error) {
	var mp3Files []string

	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(info.Name()) == ".mp3" {
			mp3Files = append(mp3Files, path)
		}
		return nil
	})

	if err != nil {
		return "", err
	}

	// Sort files by modification time (oldest first)
	sort.Slice(mp3Files, func(i, j int) bool {
		return getFileModTime(mp3Files[i]).Before(getFileModTime(mp3Files[j]))
	})

	if len(mp3Files) > 0 {
		return mp3Files[0], nil
	}

	return "", fmt.Errorf("no MP3 files found")
}

func getFileModTime(filePath string) time.Time {
	fileInfo, _ := os.Stat(filePath)
	return fileInfo.ModTime()
}