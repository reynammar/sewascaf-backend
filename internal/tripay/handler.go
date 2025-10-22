package tripay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io" // <-- IMPORT BARU
	"log"
	"net/http"
	"sewascaf.com/api/internal/models" // <-- IMPORT BARU

	"github.com/gin-gonic/gin"
	"gorm.io/gorm" // <-- IMPORT BARU
)

type Handler struct {
	DB         *gorm.DB
	APIKey     string
	PrivateKey string
}

func NewHandler(db *gorm.DB, apiKey, privateKey string) *Handler {
	return &Handler{
		DB:         db,
		APIKey:     apiKey,
		PrivateKey: privateKey,
	}
}

// Definisikan struct agar sesuai dengan respons JSON dari Tripay
type PaymentChannel struct {
	Group string `json:"group"`
	Code  string `json:"code"`
	Name  string `json:"name"`
	Icon  string `json:"icon_url"`
}

type TripayChannelResponse struct {
	Success bool             `json:"success"`
	Message string           `json:"message"`
	Data    []PaymentChannel `json:"data"`
}

func (h *Handler) GetPaymentChannels(c *gin.Context) {
	// 1. Siapkan request ke API Sandbox Tripay
	req, err := http.NewRequest("GET", "https://tripay.co.id/api-sandbox/merchant/payment-channel", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request to Tripay"})
		return
	}

	// 2. Set Authorization Header dengan API Key kita
	req.Header.Set("Authorization", "Bearer "+h.APIKey)

	// 3. Kirim request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get response from Tripay"})
		return
	}
	defer resp.Body.Close()

	// 4. Decode respons JSON dari Tripay
	var tripayResponse TripayChannelResponse
	if err := json.NewDecoder(resp.Body).Decode(&tripayResponse); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse response from Tripay"})
		return
	}

	if !tripayResponse.Success {
		log.Printf("Tripay API Error: %s", tripayResponse.Message)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tripay returned an error", "details": tripayResponse.Message})
		return
	}

	// 5. Kirim data channel pembayaran ke frontend
	c.JSON(http.StatusOK, tripayResponse.Data)
}

func (h *Handler) CallbackHandler(c *gin.Context) {
	// 1. Ambil signature dari header
	tripaySignature := c.GetHeader("X-Callback-Signature")

	// 2. Baca body JSON dari request
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read body"})
		return
	}

	// 3. Verifikasi Signature (Langkah Keamanan Paling Penting)
	mac := hmac.New(sha256.New, []byte(h.PrivateKey))
	mac.Write(body)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	if tripaySignature != expectedSignature {
		c.JSON(http.StatusForbidden, gin.H{"error": "Invalid signature"})
		return
	}

	// 4. Decode body JSON ke sebuah map untuk dibaca
	var data map[string]interface{}
	json.Unmarshal(body, &data)

	status, ok := data["status"].(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid data format"})
		return
	}
	merchantRef, _ := data["merchant_ref"].(string) // Ini adalah Order ID kita

	// 5. Update status order di database kita
	if status == "PAID" {
		// Jika statusnya PAID, update order kita menjadi 'active'
		result := h.DB.Model(&models.Order{}).Where("id = ?", merchantRef).Update("status", "active")
		if result.Error != nil {
			log.Printf("Failed to update order status: %v", result.Error)
			// Tetap kirim 200 OK agar Tripay tidak coba kirim callback lagi
		}
	} else {
		// Jika statusnya EXPIRED atau FAILED, update menjadi 'cancelled'
		h.DB.Model(&models.Order{}).Where("id = ?", merchantRef).Update("status", "cancelled")
	}

	// 6. Kirim respons 200 OK ke Tripay untuk konfirmasi
	c.JSON(http.StatusOK, gin.H{"success": true})
}