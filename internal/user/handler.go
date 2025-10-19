package user

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"time" // REVISI: Import baru untuk JWT

	"sewascaf.com/api/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5" // REVISI: Import baru untuk JWT
	"github.com/google/uuid"

	"gorm.io/gorm"
)

// Handler sekarang menampung DB dan config Supabase
type Handler struct {
	DB                 *gorm.DB
	SupabaseURL        string
	SupabaseServiceKey string
	JWTSecret          string // REVISI: Tambahkan JWTSecret untuk membuat token baru
}

// NewHandler adalah constructor untuk membuat instance Handler baru
func NewHandler(db *gorm.DB, supabaseURL string, supabaseServiceKey string, jwtSecret string) *Handler { // REVISI: Tambahkan parameter jwtSecret
	return &Handler{
		DB:                 db,
		SupabaseURL:        supabaseURL,
		SupabaseServiceKey: supabaseServiceKey,
		JWTSecret:          jwtSecret, // REVISI: Inisialisasi JWTSecret
	}
}

// GetProfile mengambil data profil user yang sedang login
func (h *Handler) GetProfile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
		return
	}

	var user models.User
	if result := h.DB.Where("id = ?", userID).First(&user); result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// UpgradeToVendor mengubah role user menjadi 'pengusaha' dan membuat profil toko
func (h *Handler) UpgradeToVendor(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
		return
	}
	
	file, err := c.FormFile("shop_profile_image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Shop profile image is required"})
		return
	}

	shopName := c.PostForm("shop_name")
	shopAddress := c.PostForm("shop_address")
	shopPhoneNumber := c.PostForm("shop_phone_number")
	shopDescription := c.PostForm("shop_description")
	
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open file"})
		return
	}
	defer src.Close()

	fileName := fmt.Sprintf("%s-%s", uuid.New().String(), filepath.Base(file.Filename))
	uploadURL := fmt.Sprintf("%s/storage/v1/object/shop-profiles/%s", h.SupabaseURL, fileName)
	
	req, err := http.NewRequest("POST", uploadURL, src)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create upload request"})
		return
	}

	req.Header.Set("Authorization", "Bearer "+h.SupabaseServiceKey)
	req.Header.Set("Content-Type", file.Header.Get("Content-Type"))
	
	log.Printf("MANUAL UPLOAD: Sending request to %s", uploadURL)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("!!! MANUAL UPLOAD FAILED (Network Error) !!! Error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to execute upload request"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("!!! MANUAL UPLOAD FAILED (Supabase Error) !!! Status: %s, Body: %s", resp.Status, string(body))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Supabase rejected the file upload", "details": string(body)})
		return
	}
	log.Println("✅ Manual upload to Supabase successful.")

	imageURL := fmt.Sprintf("%s/storage/v1/object/public/shop-profiles/%s", h.SupabaseURL, fileName)

	var user models.User
	err = h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", userID).First(&user).Error; err != nil {
			return errors.New("user not found")
		}

		if user.Role != "user" {
			return errors.New("user is already a vendor or has a different role")
		}

		if err := tx.Model(&user).Update("role", "pengusaha").Error; err != nil {
			return err
		}

		newShop := models.Shop{
			ID:                  uuid.New(),
			UserID:              user.ID,
			ShopName:            shopName,
			ShopAddress:         shopAddress,
			ShopPhoneNumber:     shopPhoneNumber,
			ShopDescription:     shopDescription,
			ShopProfileImageURL: imageURL,
		}
		if err := tx.Create(&newShop).Error; err != nil {
			return err
		}
		
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// REVISI: Buat token JWT baru setelah role berhasil diubah
	claims := jwt.MapClaims{
		"sub": user.ID,
		"exp": time.Now().Add(time.Hour * 24).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	newTokenString, err := token.SignedString([]byte(h.JWTSecret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate new token after role upgrade"})
		return
	}

	// REVISI: Kirim token baru di dalam respons
	c.JSON(http.StatusOK, gin.H{
		"message":   "Successfully upgraded to vendor. Please use the new token.",
		"new_token": newTokenString,
	})
}