package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	UPLOAD_DIR = "/app/uploads"
	MAX_FILE_SIZE = 50 << 20 // 50MB
)

func main() {
	// Get API key from environment
	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		log.Fatal("API_KEY environment variable is required")
	}

	// Create upload directory if it doesn't exist
	if err := os.MkdirAll(UPLOAD_DIR, 0755); err != nil {
		log.Fatal("Failed to create upload directory:", err)
	}

	// Pass API key to handlers via closure
	http.HandleFunc("/upload", uploadHandlerWithKey(apiKey))
	http.HandleFunc("/health", healthHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	log.Printf("Upload directory: %s", UPLOAD_DIR)
	log.Printf("API key configured: %s", maskAPIKey(apiKey))
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func uploadHandlerWithKey(apiKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uploadHandler(w, r, apiKey)
	}
}

func uploadHandler(w http.ResponseWriter, r *http.Request, apiKey string) {
	// Only allow POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validate API key from query parameters
	requestAPIKey := r.URL.Query().Get("api_key")
	if requestAPIKey != apiKey {
		http.Error(w, "Invalid API key", http.StatusUnauthorized)
		return
	}

	// Parse multipart form with size limit
	r.Body = http.MaxBytesReader(w, r.Body, MAX_FILE_SIZE)
	if err := r.ParseMultipartForm(MAX_FILE_SIZE); err != nil {
		http.Error(w, "File too large or invalid form", http.StatusBadRequest)
		return
	}

	// Get the uploaded file
	file, header, err := r.FormFile("epub")
	if err != nil {
		http.Error(w, "Failed to get uploaded file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate file extension
	filename := header.Filename
	if !strings.HasSuffix(strings.ToLower(filename), ".epub") {
		http.Error(w, "File must be an EPUB", http.StatusBadRequest)
		return
	}

	// Generate unique filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	safeFilename := fmt.Sprintf("%s_%s", timestamp, filepath.Base(filename))
	destPath := filepath.Join(UPLOAD_DIR, safeFilename)

	// Create destination file
	destFile, err := os.Create(destPath)
	if err != nil {
		log.Printf("Failed to create destination file: %v", err)
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	defer destFile.Close()

	// Copy uploaded file to destination
	bytesWritten, err := io.Copy(destFile, file)
	if err != nil {
		log.Printf("Failed to copy file: %v", err)
		os.Remove(destPath) // Clean up partial file
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully uploaded EPUB: %s (%d bytes)", safeFilename, bytesWritten)

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status": "success", "filename": "%s", "size": %d}`, safeFilename, bytesWritten)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"status": "healthy"}`)
}

func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}
