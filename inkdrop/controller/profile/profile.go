package profile

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"inkdrop/config"
	"inkdrop/model"

	"github.com/nfnt/resize"
)

// GetProfile retrieves the profile of a user
func GetProfile(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("user")
	if username == "" {
		http.Error(w, "User parameter required", http.StatusBadRequest)
		return
	}

	db := config.GetDB()
	user, err := model.GetUserByName(username, db)
	if err != nil || user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"name":"%s","bio":"%s","pfp":"%s"}`, user.Name, escapeJSON(user.Bio), user.PFP)
}

// UpdateProfile updates the user's bio
func UpdateProfile(w http.ResponseWriter, r *http.Request) {
	ss := config.GetSessionManager()
	isLoggedIn := ss.GetBool(r.Context(), "isLoggedIn")
	username := ss.GetString(r.Context(), "name")

	if !isLoggedIn || username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	r.ParseForm()
	bio := strings.TrimSpace(r.FormValue("bio"))

	// Validate bio length (max 500 characters)
	if len(bio) > 500 {
		http.Error(w, "Bio too long (max 500 characters)", http.StatusBadRequest)
		return
	}

	db := config.GetDB()
	user, err := model.GetUserByName(username, db)
	if err != nil || user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Update bio in database
	ctx := context.Background()
	_, err = db.NewUpdate().
		Model(user).
		Set("bio = ?", bio).
		Where("id = ?", user.ID).
		Exec(ctx)

	if err != nil {
		http.Error(w, "Failed to update profile", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"success":true}`)
}

// UploadProfilePicture handles profile picture upload with auto-crop and resize
func UploadProfilePicture(w http.ResponseWriter, r *http.Request) {
	ss := config.GetSessionManager()
	isLoggedIn := ss.GetBool(r.Context(), "isLoggedIn")
	username := ss.GetString(r.Context(), "name")

	if !isLoggedIn || username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse form with max 10MB file size
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("pfp")
	if err != nil {
		http.Error(w, "No file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate file type
	contentType := handler.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		http.Error(w, "Invalid file type (must be image)", http.StatusBadRequest)
		return
	}

	// Read and decode image
	fileData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	var img image.Image
	if strings.Contains(contentType, "png") {
		img, err = png.Decode(bytes.NewReader(fileData))
	} else if strings.Contains(contentType, "jpeg") || strings.Contains(contentType, "jpg") {
		img, err = jpeg.Decode(bytes.NewReader(fileData))
	} else {
		http.Error(w, "Unsupported image format", http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, "Failed to decode image", http.StatusBadRequest)
		return
	}

	db := config.GetDB()
	user, err := model.GetUserByName(username, db)
	if err != nil || user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Auto-crop to square and resize to 200x200
	croppedImg := squareCrop(img)
	resizedImg := resize.Resize(200, 200, croppedImg, resize.Lanczos3)

	// Generate filename
	filename := fmt.Sprintf("%d_%d.jpg", user.ID, time.Now().Unix())
	dirPath := filepath.Join("_meta", "pfp")
	filePath := filepath.Join(dirPath, filename)

	// Ensure directory exists
	os.MkdirAll(dirPath, 0755)

	// Save old profile picture path for deletion
	oldPFP := user.PFP

	// Save new image
	out, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "Failed to save image", http.StatusInternalServerError)
		return
	}
	defer out.Close()

	err = jpeg.Encode(out, resizedImg, nil)
	if err != nil {
		http.Error(w, "Failed to encode image", http.StatusInternalServerError)
		return
	}

	// Update database
	newPFPPath := fmt.Sprintf("/_meta/pfp/%s", filename)
	ctx := context.Background()
	_, err = db.NewUpdate().
		Model(user).
		Set("pfp = ?", newPFPPath).
		Where("id = ?", user.ID).
		Exec(ctx)

	if err != nil {
		http.Error(w, "Failed to update profile", http.StatusInternalServerError)
		return
	}

	// Delete old profile picture if it's not the default
	if oldPFP != "/assets/default.png" && !strings.HasPrefix(oldPFP, "/assets/") {
		os.Remove("." + oldPFP)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"success":true,"pfp":"%s"}`, newPFPPath)
}

// Helper function to auto-crop image to square
func squareCrop(img image.Image) image.Image {
	bounds := img.Bounds()
	width := bounds.Max.X - bounds.Min.X
	height := bounds.Max.Y - bounds.Min.Y

	var size int
	var x, y int

	if width < height {
		size = width
		y = (height - width) / 2
	} else {
		size = height
		x = (width - height) / 2
	}

	return img.(interface {
		SubImage(image.Rectangle) image.Image
	}).SubImage(image.Rect(x, y, x+size, y+size))
}

func escapeJSON(s string) string {
	return strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
		"\r", `\r`,
		"\t", `\t`,
	).Replace(s)
}
