// Lokasi: internal/chatbot/handler.go
package chatbot

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/generative-ai-go/genai"
	"github.com/google/uuid"
	"google.golang.org/api/option"
	"gorm.io/gorm"
	"sewascaf.com/api/internal/models"
)

type Handler struct {
	DB           *gorm.DB
	GeminiClient *genai.GenerativeModel
}

func NewHandler(db *gorm.DB, apiKey string) *Handler {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatalf("Failed to create Gemini client: %v", err)
	}
	model := client.GenerativeModel("gemini-2.5-flash")

	return &Handler{
		DB:           db,
		GeminiClient: model,
	}
}

// Struct untuk menampung pertanyaan dari user
type AskPayload struct {
	Question string `json:"question" binding:"required"`
}

func (h *Handler) AskChatbot(c *gin.Context) {
	var payload AskPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body, 'question' is required"})
		return
	}

	ctx := context.Background()
	
	// Kirim pertanyaan ke Gemini
	resp, err := h.GeminiClient.GenerateContent(ctx, genai.Text(payload.Question))
	if err != nil {
		log.Printf("Gemini API call failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get response from AI"})
		return
	}

	// Ambil teks jawaban dari respons
	var answer string
	if len(resp.Candidates) > 0 {
		if content := resp.Candidates[0].Content; content != nil {
			if part := content.Parts[0]; part != nil {
				answer = string(part.(genai.Text))
			}
		}
	}

	if answer == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI returned an empty response"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"answer": answer})
}

func (h *Handler) saveAndCleanHistory(userID, question, answer string) {
	const maxHistory = 60

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		// Simpan percakapan baru
		newHistory := models.ChatHistory{
			ID:       uuid.New(),
			UserID:   uuid.MustParse(userID),
			Question: question,
			Answer:   answer,
		}
		if err := tx.Create(&newHistory).Error; err != nil {
			return err
		}

		// Hitung jumlah riwayat user saat ini
		var count int64
		tx.Model(&models.ChatHistory{}).Where("user_id = ?", userID).Count(&count)

		// Jika melebihi batas, hapus yang paling lama
		if count > maxHistory {
			var oldestHistory models.ChatHistory
			// Cari 1 entri paling lama
			tx.Where("user_id = ?", userID).Order("created_at asc").First(&oldestHistory)
			// Hapus entri tersebut
			tx.Delete(&oldestHistory)
		}

		return nil
	})

	if err != nil {
		log.Printf("Failed to save or clean chat history for user %s: %v", userID, err)
	}
}