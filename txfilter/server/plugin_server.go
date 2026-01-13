package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	defaultHost         = "localhost"
	defaultPort         = "8080"
	pluginEndpoint      = "/suspicious_txfilter.so"
	bundleEndpoint      = "/suspicious_txfilter.so.bundle"
	metadataEndpoint    = "/suspicious_txfilter.json"
	defaultPluginFile   = "suspicious_txfilter.so"
	defaultBundleFile   = "suspicious_txfilter.so.bundle"
	defaultMetadataFile = "suspicious_txfilter.json"
)

// loggingMiddleware logs HTTP requests with method, path, status, and duration
func loggingMiddleware(next http.HandlerFunc, logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer wrapper to capture status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next(rw, r)

		duration := time.Since(start)
		logger.Printf("[%s] %s %s %d %v",
			r.RemoteAddr,
			r.Method,
			r.URL.Path,
			rw.statusCode,
			duration,
		)
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func main() {
	var (
		pluginFile   = flag.String("plugin", defaultPluginFile, "Path to the plugin .so file")
		bundleFile   = flag.String("bundle", defaultBundleFile, "Path to the signed bundle file")
		metadataFile = flag.String("metadata", defaultMetadataFile, "Path to the metadata JSON file")
		host         = flag.String("host", defaultHost, "Host to bind to (empty string binds to all interfaces)")
		port         = flag.String("port", defaultPort, "Port to listen on")
		dir          = flag.String("dir", ".", "Directory to serve files from (if relative paths are used)")
		logFile      = flag.String("log", "", "Path to log file (empty means stdout/stderr)")
	)
	flag.Parse()

	// Setup logger
	var logWriter io.Writer = os.Stderr
	if *logFile != "" {
		file, err := os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			log.Fatalf("Failed to open log file: %v", err)
		}
		// Don't close the file - keep it open for the lifetime of the server
		logWriter = file
	}

	logger := log.New(logWriter, "", log.LstdFlags|log.Lmicroseconds)

	if *logFile != "" {
		logger.Printf("Logging to file: %s", *logFile)
	}

	// Get absolute paths
	pluginFilePath := filepath.Join(*dir, *pluginFile)
	bundleFilePath := filepath.Join(*dir, *bundleFile)
	metadataFilePath := filepath.Join(*dir, *metadataFile)

	// Verify files exist
	if _, err := os.Stat(pluginFilePath); os.IsNotExist(err) {
		logger.Fatalf("Plugin file not found: %s", pluginFilePath)
	}
	if _, err := os.Stat(bundleFilePath); os.IsNotExist(err) {
		logger.Fatalf("Bundle file not found: %s", bundleFilePath)
	}
	if _, err := os.Stat(metadataFilePath); os.IsNotExist(err) {
		logger.Fatalf("Metadata file not found: %s", metadataFilePath)
	}

	// Setup HTTP handlers with logging middleware
	http.HandleFunc(pluginEndpoint, loggingMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		http.ServeFile(w, r, pluginFilePath)
	}, logger))

	http.HandleFunc(bundleEndpoint, loggingMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		http.ServeFile(w, r, bundleFilePath)
	}, logger))

	http.HandleFunc(metadataEndpoint, loggingMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		http.ServeFile(w, r, metadataFilePath)
	}, logger))

	// Health check endpoint
	http.HandleFunc("/health", loggingMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	}, logger))

	// Construct address
	var addr string
	if *host == "" {
		addr = ":" + *port
	} else {
		addr = *host + ":" + *port
	}

	// Determine display host for logs
	displayHost := *host
	if displayHost == "" {
		displayHost = "0.0.0.0"
	}

	logger.Printf("Starting plugin server on %s", addr)
	logger.Printf("Serving plugin file: %s", pluginFilePath)
	logger.Printf("Serving bundle file: %s", bundleFilePath)
	logger.Printf("Serving metadata file: %s", metadataFilePath)
	logger.Printf("Plugin endpoint: http://%s:%s%s", displayHost, *port, pluginEndpoint)
	logger.Printf("Bundle endpoint: http://%s:%s%s", displayHost, *port, bundleEndpoint)
	logger.Printf("Metadata endpoint: http://%s:%s%s", displayHost, *port, metadataEndpoint)

	if err := http.ListenAndServe(addr, nil); err != nil {
		logger.Fatalf("Server failed: %v", err)
	}
}
