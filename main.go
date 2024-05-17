package main

import (
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
)

const (
	kChunkSize             = 32_000
	kContentTypeSampleSize = 512
	kEmptyString           = ""
)

type MyTask struct {
	Name string `json:"name"`
}

func main() {

	mux := chi.NewRouter()
	mux.Use(middleware.Logger)

	// curl http://localhost:8080/ping
	mux.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "pong\n")
	})

	// curl 'http://localhost:8080/task' --header 'Content-Type: application/json' --data '{"name": "start the car"}'
	mux.Post("/task", func(w http.ResponseWriter, r *http.Request) {
		var req MyTask
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			fmt.Fprintf(w, "Task Request: %#v\n", req)
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
	})

	// curl 'http://localhost:8080/upload' --form 'afile=@"/Users/mike/Downloads/ARRL_Handbook_Setup_2023.zip"'
	mux.Post("/upload", func(w http.ResponseWriter, r *http.Request) {
		fileUpload(w, r)
	})

	// curl --remote-name http://localhost:8080/download/ARRL_Handbook_Setup_2023.zip
	mux.Get("/download/{filename}", func(w http.ResponseWriter, r *http.Request) {
		fileDownload(w, r)
	})

	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		panic(err)
	}
}

// fileUpload performs a chunked upload of the file to the service.
func fileUpload(w http.ResponseWriter, r *http.Request) {
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}(r.Body)

	err := r.ParseForm()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	file, fileHeader, err := r.FormFile("afile")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer func(file multipart.File) {
		err := file.Close()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}(file)

	out, err := os.Create(fileHeader.Filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer func(out *os.File) {
		err := out.Close()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}(out)

	// Provide a buffer to optimize processing of chunks.
	// If using a buffer of 32KB is acceptable, you can use io.Copy().
	buf := make([]byte, kChunkSize)
	if l, err := io.CopyBuffer(out, file, buf); err == nil {
		fmt.Fprintf(w, "UPLOADED: File '%s' with %d bytes\n", fileHeader.Filename, l)
	} else {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// fileDownload performs a chunked download of the file from the service.
func fileDownload(w http.ResponseWriter, r *http.Request) {
	filename := chi.URLParam(r, "filename")
	file, err := os.Open(filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}(file)

	// Detect file content type using the first 512 bytes of file date.
	if fileContentType, err := getFileContentType(file); err == nil {
		if fileSize, err := getFileSize(file); err == nil {
			// Set the HTTP response headers.
			w.Header().Set("Content-Type", fileContentType+";"+filename)
			w.Header().Set("Content-Length", fileSize)

			// Provide a buffer to optimize processing of chunks.
			// If using a buffer of 32KB is acceptable, you can use io.Copy().
			buf := make([]byte, kChunkSize)
			if _, err := io.CopyBuffer(w, file, buf); err == nil {
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	} else {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// getFileSize determines the number of bytes in the file and returns a string of the length.
func getFileSize(file *os.File) (string, error) {
	if fileStat, err := file.Stat(); err == nil {
		return strconv.FormatInt(fileStat.Size(), 10), nil
	} else {
		return kEmptyString, err
	}
}

// getFileContentType determines the kind of file based reading the first 512 bytes from the file.
func getFileContentType(file *os.File) (string, error) {
	content := make([]byte, kContentTypeSampleSize)
	if offset, err := file.Seek(0, io.SeekCurrent); err == nil {
		if _, err := file.Read(content); err == nil {
			fileContentType := http.DetectContentType(content)
			// Rewind the file to its original location.
			_, err = file.Seek(offset, 0)
			return fileContentType, err
		} else {
			return kEmptyString, err
		}
	} else {
		return kEmptyString, err
	}
}
