package main

import (
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(writer http.ResponseWriter, req *http.Request) {
	const uploadLimit = 1 << 30
	req.Body = http.MaxBytesReader(writer, req.Body, uploadLimit)

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

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(writer, http.StatusInternalServerError, "couldn't get video metadata from db", err)
		return
	}
	if userID != video.UserID {
		respondWithError(writer, http.StatusUnauthorized, "unauthorized user trying to upload video thumbnail", err)
		return
	}

	videoFile, fileHeader, err := req.FormFile("video")
	if err != nil {
		respondWithError(writer, http.StatusInternalServerError, "couldn't get file data from form", err)
		return
	}
	defer videoFile.Close()

	mediaType, _, err := mime.ParseMediaType(fileHeader.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(writer, http.StatusInternalServerError, "couldn't get mime type from Header", err)
		return
	}
	if mediaType != "video/mp4" {
		respondWithError(writer, http.StatusBadRequest, "unsupported media type uploaded", nil)
		return
	}

	tempFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(writer, http.StatusInternalServerError, "couldn't create temp file on disk", err)
		return
	}

	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	if _, err = io.Copy(tempFile, videoFile); err != nil {
		respondWithError(writer, http.StatusInternalServerError, "couldn't write image to disk", err)
		return
	}

	_, err = tempFile.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(writer, http.StatusInternalServerError, "Could not reset file pointer", err)
		return
	}

	aspectRatio, err := getVideoAspectRatio(tempFile.Name())
	if err != nil {
		respondWithError(writer, http.StatusInternalServerError, "couldn't get video's aspect ratio", err)
	}

	key := getVideoOrientationFromAspectRatio(aspectRatio) + "/" + getAssetPath(mediaType)
	_, err = cfg.s3Client.PutObject(req.Context(), &s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),
		Key:         aws.String(key),
		Body:        tempFile,
		ContentType: aws.String(mediaType),
	})

	if err != nil {
		respondWithError(writer, http.StatusInternalServerError, "Error uploading file to S3", err)
		return
	}

	videoURL := cfg.getObjectURL(key)
	video.VideoURL = &videoURL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(writer, http.StatusInternalServerError, "couldn't update video thumbnail db", err)
		return
	}

	respondWithJSON(writer, http.StatusOK, video)
}
