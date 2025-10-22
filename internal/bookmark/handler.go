package bookmark

import (
	"errors"
	"net/http"

	"sewascaf.com/api/internal/models"
	"sewascaf.com/api/internal/product"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Handler struct {
	DB *gorm.DB
}

func NewHandler(db *gorm.DB) *Handler {
	return &Handler{DB: db}
}

func (h *Handler) AddBookmark(c *gin.Context) {
	productID := c.Param("productId")

	userIDInterface, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}
	userIDString, ok := userIDInterface.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
		return
	}

	session := h.DB.Session(&gorm.Session{PrepareStmt: false})

	err := session.Transaction(func(tx *gorm.DB) error {
		var productCount int64
		if err := tx.Model(&models.Product{}).Where("id = ?", productID).Count(&productCount).Error; err != nil {
			return err
		}
		if productCount == 0 {
			return errors.New("product not found")
		}

		var bookmarkCount int64
		if err := tx.Model(&models.Bookmark{}).Where("user_id = ? AND product_id = ?", userIDString, productID).Count(&bookmarkCount).Error; err != nil {
			return err
		}
		if bookmarkCount > 0 {
			return errors.New("product already bookmarked")
		}

		newBookmark := models.Bookmark{
			ID:        uuid.New(),
			UserID:    uuid.MustParse(userIDString),
			ProductID: uuid.MustParse(productID),
		}
		return tx.Create(&newBookmark).Error
	})

	if err != nil {
		if err.Error() == "product already bookmarked" { c.JSON(http.StatusConflict, gin.H{"error": err.Error()}); return }
		if err.Error() == "product not found" { c.JSON(http.StatusNotFound, gin.H{"error": err.Error()}); return }
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to bookmark product", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Product bookmarked successfully"})
}

func (h *Handler) DeleteBookmark(c *gin.Context) {
	productID := c.Param("productId")

	userIDInterface, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}
	userIDString, ok := userIDInterface.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
		return
	}

	result := h.DB.Where("user_id = ? AND product_id = ?", userIDString, productID).Delete(&models.Bookmark{})

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete bookmark", "details": result.Error.Error()})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Bookmark not found for this product"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) GetUserBookmarks(c *gin.Context) {
	userIDInterface, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}
	userIDString, ok := userIDInterface.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
		return
	}

	// Siapkan slice untuk menampung hasil akhir (mirip DTO di GetProducts)
	var response []product.ProductListResponse // Kita "pinjam" DTO dari paket product

	// Query ini sedikit kompleks:
	// 1. Mulai dari tabel bookmarks
	// 2. Filter berdasarkan user_id
	// 3. Gabungkan (JOIN) dengan products, shops, dan reviews
	// 4. Hitung rata-rata rating
	err := h.DB.Table("bookmarks").
		Select(`
			products.id, 
			products.name, 
			products.price_per_day, 
			products.discount_price_per_day, 
			products.image_url, 
			shops.shop_name, 
			COALESCE(AVG(reviews.rating), 0) as average_rating
		`).
		Joins("JOIN products ON products.id = bookmarks.product_id").
		Joins("JOIN shops ON shops.id = products.shop_id").
		Joins("LEFT JOIN reviews ON reviews.product_id = bookmarks.product_id").
		Where("bookmarks.user_id = ?", userIDString).
		Group("products.id, shops.shop_name").
		Scan(&response).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve bookmarks", "details": err.Error()})
		return
	}

	// Jika tidak ada bookmark, kembalikan array kosong
	if response == nil {
		response = make([]product.ProductListResponse, 0)
	}

	c.JSON(http.StatusOK, response)
}