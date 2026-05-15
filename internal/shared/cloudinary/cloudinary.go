package cloudinary

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/logger"
)

type Config struct {
	CloudName string
	APIKey    string
	APISecret string
}

var cloudCfg Config

// Load initializes Cloudinary configuration from environment
func Load() {
	cloudCfg = Config{
		CloudName: getEnv("CLOUDINARY_CLOUD_NAME", ""),
		APIKey:    getEnv("CLOUDINARY_API_KEY", ""),
		APISecret: getEnv("CLOUDINARY_API_SECRET", ""),
	}
}

// IsConfigured returns true if Cloudinary is properly configured
func IsConfigured() bool {
	return cloudCfg.CloudName != "" && cloudCfg.APIKey != "" && cloudCfg.APISecret != ""
}

// UploadProfilePicture uploads a profile picture and returns the URL
// MAX_FILE_SIZE: 5MB (5 * 1024 * 1024 bytes)
const MaxFileSize = 5 * 1024 * 1024

func UploadProfilePicture(ctx context.Context, imageData []byte, userPublicID string) (string, error) {
	if !IsConfigured() {
		logger.Warn("Cloudinary upload skipped - not configured")
		return "", fmt.Errorf("cloudinary is not configured")
	}

	// Validate file size
	if len(imageData) > MaxFileSize {
		logger.Warn("Cloudinary upload failed - file too large",
			logger.Int("size", len(imageData)),
			logger.Int("max_size", MaxFileSize),
		)
		return "", fmt.Errorf("file size exceeds maximum limit of 5MB")
	}

	// Create Cloudinary client
	cld, err := cloudinary.NewFromParams(cloudCfg.CloudName, cloudCfg.APIKey, cloudCfg.APISecret)
	if err != nil {
		logger.Error("Failed to create Cloudinary client", logger.Err(err))
		return "", fmt.Errorf("failed to create Cloudinary client: %w", err)
	}

	// Convert to base64 data URI (Cloudinary expects this format)
	// Supported formats: JPEG, PNG, WebP
	dataURI := fmt.Sprintf("data:image/jpeg;base64,%s", base64.StdEncoding.EncodeToString(imageData))

	// Upload with transformations for profile picture
	overwrite := true
	uploadResult, err := cld.Upload.Upload(ctx, dataURI, uploader.UploadParams{
		Folder:         "cinematch/profiles",
		PublicID:       userPublicID,
		Overwrite:      &overwrite,
		Format:         "jpg",                                   // Convert to JPEG for consistency
		Transformation: "c_fill,g_face,h_400,w_400,q_auto:good", // Square crop, face-centered, good quality
	})
	if err != nil {
		logger.Error("Failed to upload image to Cloudinary",
			logger.String("public_id", userPublicID),
			logger.Err(err),
		)
		return "", fmt.Errorf("failed to upload image: %w", err)
	}

	logger.Debug("Profile picture uploaded to Cloudinary",
		logger.String("public_id", userPublicID),
		logger.String("url", uploadResult.SecureURL),
	)
	return uploadResult.SecureURL, nil
}

func getEnv(key, defaultVal string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultVal
}

// DeleteProfilePicture removes a profile picture from Cloudinary
func DeleteProfilePicture(ctx context.Context, publicID string) error {
	if !IsConfigured() {
		return fmt.Errorf("cloudinary is not configured")
	}

	cld, err := cloudinary.NewFromParams(cloudCfg.CloudName, cloudCfg.APIKey, cloudCfg.APISecret)
	if err != nil {
		return fmt.Errorf("failed to create Cloudinary client: %w", err)
	}

	_, err = cld.Upload.Destroy(ctx, uploader.DestroyParams{
		PublicID: publicID,
	})
	return err
}
