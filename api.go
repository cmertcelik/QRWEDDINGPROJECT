package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"mime"

	"github.com/gorilla/mux"
)

// Basit token saklama (gerçek uygulamada DB veya cache kullanılmalı)
var tokens = map[string]bool{}

// Token üret
func generateToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Token endpoint
func tokenHandler(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	token := generateToken()
	tokens[token] = true
	w.Write([]byte(token))
}

// Disk alanı kontrol endpoint'i
func diskHandler(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var stat syscall.Statfs_t
	err := syscall.Statfs(".", &stat)
	if err != nil {
		// örnek bir hata cevabı...
		setCORSHeaders(w)
		http.Error(w, "Disk kontrol edilemedi", http.StatusInternalServerError)
		return
	}
	// Boş alan byte cinsinden
	free := stat.Bavail * uint64(stat.Bsize)
	// Örneğin 1GB'dan az ise busy dön
	if free < 1*1024*1024*1024 {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"busy"}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

// Dosya yükleme endpoint
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Token kontrolü
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	token := strings.TrimPrefix(auth, "Bearer ")
	if !tokens[token] {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	// Maksimum 5GB dosya (tek seferde)
	r.ParseMultipartForm(5 << 30)

	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Dosya alınamadı", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Sadece fotoğraf veya video dosyası kabul et
	ext := strings.ToLower(filepath.Ext(handler.Filename))
	mimeType := mime.TypeByExtension(ext)
	if !strings.HasPrefix(mimeType, "image/") && !strings.HasPrefix(mimeType, "video/") {
		http.Error(w, "Sadece fotoğraf veya video yükleyebilirsiniz", http.StatusBadRequest)
		return
	}

	// uploads klasörünü oluştur
	os.MkdirAll("uploads", os.ModePerm)

	// Dosya ismini uniq yap
	uniqueName := fmt.Sprintf("%s_%s", generateToken(), handler.Filename)
	dst, err := os.Create("uploads/" + uniqueName)
	if err != nil {
		http.Error(w, "Dosya kaydedilemedi", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	io.Copy(dst, file)

	fmt.Fprintf(w, "Dosya yüklendi: %s", uniqueName)
}

// CORS başlıklarını ekleyen yardımcı fonksiyon
func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "yourdomain.com")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	http.Error(w, "Not found", http.StatusNotFound)
}

func main() {
	r := mux.NewRouter()

	r.HandleFunc("/disk", diskHandler).Methods("GET", "OPTIONS")
	r.HandleFunc("/token", tokenHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/upload", uploadHandler).Methods("POST", "OPTIONS")
	r.NotFoundHandler = http.HandlerFunc(notFoundHandler)

	fmt.Println("API çalışıyor :8443 (HTTPS)")

	// err := http.ListenAndServeTLS(":8443", "/etc/letsencrypt/archive/yourdomain.com/fullchain1.pem", "/etc/letsencrypt/archive/yourdomain.com/privkey1.pem", r)
	// if err != nil {
	// 	fmt.Println("HTTPS başlatılamadı:", err)
	// }
}
