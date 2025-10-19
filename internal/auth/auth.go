// Lokasi: internal/auth/auth.go

package auth

import (
	"log"
	"net/http"
	"time"

	"sewascaf.com/api/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm" // <-- IMPORT BARU
)

// Handler adalah struct yang akan menampung dependensi seperti koneksi DB
type Handler struct {
	DB *gorm.DB
	JWTSecret string
}

// NewHandler adalah "constructor" untuk membuat instance Handler baru
func NewHandler(db *gorm.DB, jwtSecret string) *Handler {
	return &Handler{
		DB:        db,
		JWTSecret: jwtSecret,
	}
}

type LoginPayload struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (h *Handler) Login(c *gin.Context) {
	var payload LoginPayload
	var user models.User

	// 1. Bind JSON request ke payload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// 2. Cari user di database berdasarkan email
	if result := h.DB.Where("email = ?", payload.Email).First(&user); result.Error != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	// 3. Bandingkan password dari request dengan hash di database
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(payload.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	// 4. Jika password cocok, buat JWT Token
	// Tentukan 'claims' atau data yang akan dimasukkan ke dalam token
	claims := jwt.MapClaims{
		"sub": user.ID,                              // Subject (identitas user)
		"exp": time.Now().Add(time.Hour * 24).Unix(), // Waktu kedaluwarsa (24 jam)
	}

	// Buat token dengan claims dan metode signing HS256
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Tandatangani token dengan secret key kita
	tokenString, err := token.SignedString([]byte(h.JWTSecret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// 5. Kirim token sebagai response
	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
		"token":   tokenString,
	})
}

type RegisterPayload struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Pekerjaan string `json:"pekerjaan" binding:"required"`
	Alamat   string `json:"alamat" binding:"required"`
	Telepon  string `json:"telepon" binding:"required"`
	Role     string `json:"role" binding:"required"`
}

// Register sekarang adalah method dari struct Handler
func (h *Handler) Register(c *gin.Context) {
	var payload RegisterPayload

	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(payload.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	newUser := models.User{
		ID:       uuid.New(),
		Name:     payload.Name,
		Email:    payload.Email,
		Password: string(hashedPassword),
		Pekerjaan: payload.Pekerjaan,
		Alamat:   payload.Alamat,
		Telepon:  payload.Telepon,
		Role:     payload.Role,
	}

	if result := h.DB.Create(&newUser); result.Error != nil {
    log.Printf("!!! DATABASE CREATE FAILED !!! Error: %v", result.Error)
    c.JSON(http.StatusInternalServerError, gin.H{
        "error":   "Failed to create user",
        "details": result.Error.Error(),
    })
    return
}

	c.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully",
		"user": gin.H{
			"id":    newUser.ID,
			"name":  newUser.Name,
			"email": newUser.Email,
			"pekerjaan": newUser.Pekerjaan,
			"alamat":   newUser.Alamat,
			"telepon":  newUser.Telepon,
			"role":  newUser.Role,
		},
	})
}