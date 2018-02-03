package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"errors"
	"os/signal"
	"syscall"
	sha1lib "crypto/sha1"
	"encoding/hex"
	"time"
	"net/http"
	"sync"
	"os/exec"
	"strings"
)

type Event struct {
	File       string // Path to temp file fetched from Import.Source, will be deleted afterwards
	Target     DSN    // Sentry DSN
	ImportName string // Name of import (used for logging)
}

type Import struct {
	Name       string `json:"name"`   // Name used in logs
	Source     string `json:"source"` // rsync-compatible path (user@host:/path)
	SourceLock *sync.Mutex            // Lock target directory for reads/writes
	Target     DSN    `json:"target"` // Sentry DSN
	cache      string                 // Path to cache directory
}

type Config struct {
	Imports []Import `json:"imports"`

	// Path to the cache directory
	Cache string `json:"cache"`

	// Minimum time between two executions of the same import
	MinTimeBetweenImports uint32 `json:"minTimeBetweenImports"`

	// Maximum number of imports to run at the same time
	NumImportWorkers uint `json:"numImportWorkers"`

	// Maximum number of submissions to run at the same time
	NumSubmitWorkers uint `json:"numSubmitWorkers"`
}

const (
	eventFileExtension        = ".sentry_report"
	minTimeBetweenImportsUnit = time.Second
	sentryHTTPTimeout         = time.Second * 4
	maxTimeToWaitUntilExit    = sentryHTTPTimeout + (time.Second * 4)
)

var (
	eventQueue            chan Event
	eventQueueStopSignal  = false
	eventQueueWaitGroup   sync.WaitGroup
	importQueue           chan Import
	importQueueStopSignal = false
	importQueueWaitGroup  sync.WaitGroup
)

func (imprt *Import) Cache() string {
	if _, err := os.Stat(imprt.cache); err != nil {
		if err := os.Mkdir(imprt.cache, os.ModePerm); err != nil {
			panic(err)
		}
	}
	return imprt.cache
}

func (imprt *Import) SetCache(dir string) {
	imprt.cache = filepath.Join(dir, sha1(sha1(imprt.Source)+sha1(imprt.Target.DSN)))
}

func (config *Config) LoadJSON(bytes []byte) error {
	return json.Unmarshal(bytes, config)
}

func submitEvent(event Event) {
	// Read
	reader, err := os.Open(event.File)
	if err != nil {
		log.Printf("submitEvent: error: event.File=\"%s\" event.\"%s\" error=\"%v\"", event.File, event.ImportName, err)
		return
	}
	defer reader.Close()

	// Submit
	client := &http.Client{
		Timeout: sentryHTTPTimeout,
	}
	request, err := http.NewRequest("POST", event.Target.StoreAPI, reader)
	if err != nil {
		log.Printf("submitEvent: error: event.File=\"%s\" event.ImportName=\"%s\" error=\"%v\"", event.File, event.ImportName, err)
		return
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Sentry-Auth", event.Target.AuthHeader)
	response, err := client.Do(request)

	// Evaluate response
	if err != nil {
		log.Printf("submitEvent: error: event.File=\"%s\" event.ImportName=\"%s\" error=\"%v\"", event.File, event.ImportName, err)
		return
	}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Printf("submitEvent: error: event.File=\"%s\" event.ImportName=\"%s\" error=\"%v\"", event.File, event.ImportName, err)
		return
	}
	if response.StatusCode != 200 {
		log.Printf("submitEvent: error: event.File=\"%s\" event.ImportName=\"%s\" response.StatusCode=%d error=\"unexpected response.Status\"", event.File, event.ImportName, response.StatusCode)

		// TODO: Parse JSON response
		if response.StatusCode != 403 || !strings.HasPrefix(string(body), "{\"error\":\"An event with the same ID already exists") {
			return
		}
	}
	log.Printf("submitEvent: info: event.ImportName=\"%s\": %s", event.ImportName, body)
	syscall.Unlink(event.File)
}

func submitEvents(imprt Import) {
	imprt.SourceLock.Lock()
	defer imprt.SourceLock.Unlock()
	log.Printf("submitEvents: info: rsync: imprt.Name=\"%s\"", imprt.Name)
	output, err := rsync(imprt.Source, imprt.Cache())
	if err != nil {
		log.Printf("submitEvents: error: imprt.Name=\"%s\" output=\"%s\" error=\"%v\"", imprt.Name, string(output), err)
		return
	}
	events, err := filepath.Glob(filepath.Join(imprt.Cache(), "*"+eventFileExtension))
	if err != nil {
		log.Printf("submitEvents: error: imprt.Name=\"%s\" error=\"%v\"", imprt.Name, err)
		return
	}
	for _, file := range events {
		//log.Printf("submitEvents: info: add file to event queue: imprt.Source=\"%s\"", imprt.Source)
		eventQueue <- Event{File: file, Target: imprt.Target, ImportName: imprt.Name}
	}
}

func spawnEventQueueWorkers(queue chan Event, num uint) {
	for i := uint(0); i < num; i++ {
		eventQueueWaitGroup.Add(1)
		go func() {
			defer eventQueueWaitGroup.Done()
			for {
				if eventQueueStopSignal {
					break
				}
				select {
				case event := <-queue:
					submitEvent(event)
				case <-time.After(time.Millisecond * 100):
				}
			}
		}()
	}
}

func spawnImportQueueWorkers(queue chan Import, num uint) {
	for i := uint(0); i < num; i++ {
		importQueueWaitGroup.Add(1)
		go func() {
			defer importQueueWaitGroup.Done()
			for {
				if importQueueStopSignal {
					break
				}
				select {
				case imprt := <-queue:
					submitEvents(imprt)
				case <-time.After(time.Millisecond * 100):
				}
			}
		}()
	}
}

func startImportQueueScheduler(queue chan Import, imports []Import, minTimeBetweenImports uint32) {
	go func() {
		for {
			start := time.Now()
			for _, imprt := range imports {
				queue <- imprt
			}
			nextStart := start.Add(time.Duration(int64(minTimeBetweenImports) * int64(minTimeBetweenImportsUnit)))
			diff := nextStart.Sub(start)
			time.Sleep(diff)
		}
	}()
}

func parseConfig() (Config, error) {
	// Parse flags
	file := flag.String("config", "./sentry-rsync-import.json", "Path to config file")
	flag.Parse()

	// Read config file
	js, err := ioutil.ReadFile(*file)
	if err != nil {
		return Config{}, err
	}

	// Parse js
	config := Config{}
	if err := config.LoadJSON(js); err != nil {
		return Config{}, err
	}

	// Check if cache dir is valid
	if cache, err := os.Stat(config.Cache); err != nil {
		return Config{}, err
	} else if !cache.IsDir() {
		return Config{}, errors.New("cache must be a directory")
	}

	for idx, val := range config.Imports {
		config.Imports[idx].SetCache(config.Cache) // Create cache directories for imports
		config.Imports[idx].SourceLock = &sync.Mutex{}
		if val.Source[len(val.Source)-1:] != "/" { // Ensure trailing slash
			config.Imports[idx].Source = val.Source + "/"
		}
	}

	// Return config
	return config, nil
}

func main() {
	// Load config
	var config Config
	{
		var err error
		if config, err = parseConfig(); err != nil {
			log.Fatal(err)
		}
	}

	// Create event queue and spawn event queue workers
	eventQueue = make(chan Event)
	spawnEventQueueWorkers(eventQueue, config.NumSubmitWorkers)

	// Create import queue and spawn import queue workers
	importQueue = make(chan Import)
	startImportQueueScheduler(importQueue, config.Imports, config.MinTimeBetweenImports)
	spawnImportQueueWorkers(importQueue, config.NumImportWorkers)

	// Exit if we get the signal to do so
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	log.Printf("info: received %v, stopping import queue", <-signals)
	go func() {
		time.Sleep(maxTimeToWaitUntilExit)
		log.Print("error: stopping queues took to long, exiting now")
		os.Exit(1)
	}()
	importQueueStopSignal = true
	importQueueWaitGroup.Wait()
	log.Print("info: import queue stopped, stopping event queue")
	eventQueueStopSignal = true
	eventQueueWaitGroup.Wait()
	log.Print("info: event queue stopped, exiting")
	os.Exit(0)
}

func rsync(src, dest string) ([]byte, error) {
	output, err := exec.Command(
		"rsync",
		"--compress",
		"--recursive",
		"--exclude='*/'",
		"--include='*"+eventFileExtension+"'",
		"--remove-source-files",
		src, dest).Output()
	if err != nil && err.Error() == "exit status 20" {
		return []byte(""), nil
	}
	return output, err
}

func sha1(text string) string {
	hasher := sha1lib.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}
