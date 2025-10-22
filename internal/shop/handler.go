// Lokasi: internal/shop/handler.go

package shop

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"sewascaf.com/api/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	DB *gorm.DB
}

func NewHandler(db *gorm.DB) *Handler {
	return &Handler{DB: db}
}

// GetShopPaymentChannels menampilkan metode pembayaran yang sudah dipilih oleh vendor
func (h *Handler) GetShopPaymentChannels(c *gin.Context) {
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

	var shop models.Shop
	// Kita hanya butuh satu kolom, jadi kita pakai .Select() agar lebih efisien
	if err := h.DB.Select("active_payment_channels").Where("user_id = ?", userIDString).First(&shop).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Shop not found"})
		return
	}

	// Jika vendor belum memilih channel (nilainya null), kembalikan array kosong
	if shop.ActivePaymentChannels == nil {
		c.JSON(http.StatusOK, make([]string, 0))
		return
	}

	c.JSON(http.StatusOK, shop.ActivePaymentChannels)
}


func (h *Handler) GetShopProfile(c *gin.Context) {
	userIDInterface, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
		return
	}
	userIDString, ok := userIDInterface.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format in context"})
		return
	}

	var shop models.Shop
	if err := h.DB.Where("user_id = ?", userIDString).First(&shop).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Shop profile not found for this user"})
		return
	}

	c.JSON(http.StatusOK, shop)
}

type UpdateShopPayload struct {
	ShopName        string `json:"shop_name"`
	ShopAddress     string `json:"shop_address"`
	ShopDescription string `json:"shop_description"`
}

func (h *Handler) UpdateShopProfile(c *gin.Context) {
	userIDInterface, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
		return
	}
	userIDString, ok := userIDInterface.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format in context"})
		return
	}

	var payload UpdateShopPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	var shop models.Shop
	if err := h.DB.Where("user_id = ?", userIDString).First(&shop).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Shop profile not found for this user"})
		return
	}

	updates := make(map[string]interface{})

	if payload.ShopName != "" && payload.ShopName != shop.ShopName {
		if shop.ShopNameLastUpdated != nil {
			thirtyDays := 30 * 24 * time.Hour
			if time.Since(*shop.ShopNameLastUpdated) < thirtyDays {
				c.JSON(http.StatusForbidden, gin.H{"error": "Shop name can only be changed once every 30 days"})
				return
			}
		}
		updates["shop_name"] = payload.ShopName
		now := time.Now()
		updates["shop_name_last_updated"] = &now
	}

	if payload.ShopAddress != "" {
		updates["shop_address"] = payload.ShopAddress
	}
	if payload.ShopDescription != "" {
		updates["shop_description"] = payload.ShopDescription
	}

	if len(updates) > 0 {
		if err := h.DB.Model(&shop).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update shop profile"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Shop profile updated successfully",
		"shop":    shop,
	})
}

type TopProductStat struct {
	ProductID     string  `json:"product_id"`
	ProductName   string  `json:"product_name"`
	Revenue       int     `json:"revenue"`
	RentalCount   int64   `json:"rental_count"`
	AverageRating float64 `json:"average_rating"`
}

func (h *Handler) GetShopStatistics(c *gin.Context) {
	userIDInterface, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}
	userIDString, ok := userIDInterface.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format in context"})
		return
	}

	var shop models.Shop
	if err := h.DB.Where("user_id = ?", userIDString).First(&shop).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "User does not own a shop"})
		return
	}

	period := c.DefaultQuery("period", "weekly")
	now := time.Now()
	var startTime time.Time

	switch period {
	case "daily":
		startTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "weekly":
		weekday := int(now.Weekday())
		startTime = time.Date(now.Year(), now.Month(), now.Day()-weekday, 0, 0, 0, 0, now.Location())
	case "monthly":
		startTime = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	case "yearly":
		startTime = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
	default:
		period = "weekly"
		weekday := int(now.Weekday())
		startTime = time.Date(now.Year(), now.Month(), now.Day()-weekday, 0, 0, 0, 0, now.Location())
	}
	endTime := now

	var totalRevenue int
	var completedOrders int64

	h.DB.Model(&models.Order{}).Where("shop_id = ? AND status = ? AND created_at BETWEEN ? AND ?", shop.ID, "completed", startTime, endTime).Select("COALESCE(SUM(total_price), 0)").Row().Scan(&totalRevenue)
	h.DB.Model(&models.Order{}).Where("shop_id = ? AND status = ? AND created_at BETWEEN ? AND ?", shop.ID, "completed", startTime, endTime).Count(&completedOrders)

	var topProducts []TopProductStat
	h.DB.Table("products").Select(`products.id as product_id, products.name as product_name, COALESCE(SUM(orders.total_price), 0) as revenue, COUNT(orders.id) as rental_count, COALESCE(AVG(reviews.rating), 0) as average_rating`).Joins("LEFT JOIN orders ON orders.shop_id = products.shop_id AND orders.status = 'completed' AND orders.created_at BETWEEN ? AND ?", startTime, endTime).Joins("LEFT JOIN reviews ON reviews.product_id = products.id").Where("products.shop_id = ?", shop.ID).Group("products.id").Order("revenue DESC").Limit(5).Scan(&topProducts)

	c.JSON(http.StatusOK, gin.H{
		"period":           period,
		"start_date":       startTime,
		"end_date":         endTime,
		"total_revenue":    totalRevenue,
		"completed_orders": completedOrders,
		"top_products":     topProducts,
	})
}

func (h *Handler) GetShopOrders(c *gin.Context) {
	userIDInterface, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}
	userIDString, ok := userIDInterface.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format in context"})
		return
	}

	var shop models.Shop
	if err := h.DB.Select("id").Where("user_id = ?", userIDString).First(&shop).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "User does not own a shop"})
		return
	}

	query := h.DB.Model(&models.Order{}).Where("shop_id = ?", shop.ID)
	statusFilter := c.Query("status")
	if statusFilter != "" {
		query = query.Where("status = ?", statusFilter)
	}

	var orders []models.Order
	if err := query.Order("created_at DESC").Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve orders"})
		return
	}

	if orders == nil {
		orders = make([]models.Order, 0)
	}
	c.JSON(http.StatusOK, orders)
}

type UpdateStatusPayload struct {
	Status string `json:"status" binding:"required"`
}

func (h *Handler) UpdateOrderStatus(c *gin.Context) {
	userIDInterface, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}
	userIDString, ok := userIDInterface.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format in context"})
		return
	}
	orderID := c.Param("orderId")

	var payload UpdateStatusPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	newStatus := payload.Status
	allowedStatus := map[string]bool{
		"pending": true, "active": true, "completed": true, "cancelled": true,
	}
	if !allowedStatus[newStatus] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status value"})
		return
	}

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		var shop models.Shop
		if err := tx.Select("id").Where("user_id = ?", userIDString).First(&shop).Error; err != nil {
			return errors.New("shop not found for this user")
		}

		var order models.Order
		if err := tx.Where("id = ? AND shop_id = ?", orderID, shop.ID).First(&order).Error; err != nil {
			return errors.New("order not found or you do not have permission to edit it")
		}

		if err := tx.Model(&order).Update("status", newStatus).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Order status updated successfully"})
}

type UpdatePaymentChannelsPayload struct {
	Channels []string `json:"channels"`
}

func (h *Handler) UpdatePaymentChannels(c *gin.Context) {
	userIDInterface, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}
	userIDString, ok := userIDInterface.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format in context"})
		return
	}

	var payload UpdatePaymentChannelsPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body, 'channels' must be an array of strings"})
		return
	}

	var shop models.Shop
	if err := h.DB.Where("user_id = ?", userIDString).First(&shop).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Shop not found for this user"})
		return
	}

	jsonData, err := json.Marshal(payload.Channels)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal payment channels to JSON"})
		return
	}

	if err := h.DB.Model(&shop).Update("active_payment_channels", jsonData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update payment channels"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Payment channels updated successfully"})
}