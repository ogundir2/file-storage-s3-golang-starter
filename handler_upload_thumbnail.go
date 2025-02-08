package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(writer http.ResponseWriter, req *http.Request) {
	videoIDString := req.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(writer, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(req.Header)
	if err != nil {
		respondWithError(writer, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(writer, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}


	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxMemory = 10 << 20
	req.ParseMultipartForm(maxMemory)
	file, fileHeader, err := req.FormFile("thumbnail")
	if err != nil {
		respondWithError(writer, http.StatusInternalServerError, "couldn't get file data from form", err)
		return
	}

	metadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(writer, http.StatusInternalServerError, "couldn't get video metadata from db", err)
		return
	}
	if userID != metadata.UserID {
		respondWithError(writer, http.StatusUnauthorized, "unauthorized user trying to upload video thumbnail", err)
		return
	}

	// Create the image on disk
	mediaType, _, err := mime.ParseMediaType(fileHeader.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(writer, http.StatusInternalServerError, "couldn't get mime type from Header", err)
		return
	}
	if mediaType != "image/jpeg" && mediaType != "image/png" {
		respondWithError(writer, http.StatusBadRequest, "unsupported media type uploaded", nil)
		return
	}

	extension, _ := strings.CutPrefix(mediaType, "image/")
	filename := videoIDString + "." + extension
	filePath := filepath.Join(cfg.assetsRoot, filename)
	assetFile, err := os.Create(filePath)
	if err != nil {
		respondWithError(writer, http.StatusInternalServerError, "couldn't create file on disk", err)
		return
	}

	_, err = io.Copy(assetFile, file)
	if err != nil {
		respondWithError(writer, http.StatusInternalServerError, "couldn't write image to disk", err)
		return
	}
	//

	thumbnailURL := fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, filename)
	updatedVideo := database.Video{
		ID: metadata.ID,
		CreatedAt: metadata.CreatedAt,
		UpdatedAt: metadata.UpdatedAt,
		ThumbnailURL: &thumbnailURL,
		VideoURL: metadata.VideoURL,
		CreateVideoParams: metadata.CreateVideoParams,
	}

	err = cfg.db.UpdateVideo(updatedVideo)
	if err != nil {
		respondWithError(writer, http.StatusInternalServerError, "couldn't update video thumbnail db", err)
		return
	}

	respondWithJSON(writer, http.StatusOK, updatedVideo)
}
