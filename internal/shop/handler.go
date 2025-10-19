package shop

import (
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

type UpdateShopPayload struct {
	ShopName        string `json:"shop_name"`
	ShopAddress     string `json:"shop_address"`
	ShopDescription string `json:"shop_description"`
}

func (h *Handler) UpdateShopProfile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
		return
	}

	var payload UpdateShopPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	var shop models.Shop
	if err := h.DB.Where("user_id = ?", userID).First(&shop).Error; err != nil {
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

func (h *Handler) GetShopProfile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
		return
	}

	var shop models.Shop
	if err := h.DB.Where("user_id = ?", userID).First(&shop).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Shop profile not found for this user"})
		return
	}
	c.JSON(http.StatusOK, shop)
}

type TopProductStat struct {
	ProductID      string  `json:"product_id"`
	ProductName    string  `json:"product_name"`
	Revenue        int     `json:"revenue"`
	RentalCount    int64   `json:"rental_count"`
	AverageRating  float64 `json:"average_rating"`
}

func (h *Handler) GetShopStatistics(c *gin.Context) {
	// 1. Dapatkan userID dari token untuk menemukan toko
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	var shop models.Shop
	if err := h.DB.Where("user_id = ?", userID).First(&shop).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "User does not own a shop"})
		return
	}

	// 2. Tentukan rentang waktu berdasarkan query parameter 'period'
	period := c.DefaultQuery("period", "weekly") // Default ke mingguan jika tidak ada
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

	// 3. Hitung statistik utama (Total Pendapatan & Jumlah Order Selesai)
	var totalRevenue int
	var completedOrders int64
	
	h.DB.Model(&models.Order{}).
		Where("shop_id = ? AND status = ? AND created_at BETWEEN ? AND ?", shop.ID, "completed", startTime, endTime).
		Select("COALESCE(SUM(total_price), 0)").
		Row().
		Scan(&totalRevenue)

	h.DB.Model(&models.Order{}).
		Where("shop_id = ? AND status = ? AND created_at BETWEEN ? AND ?", shop.ID, "completed", startTime, endTime).
		Count(&completedOrders)

	// 4. Hitung statistik produk teratas
	var topProducts []TopProductStat
	h.DB.Table("products").
		Select(`
			products.id as product_id,
			products.name as product_name,
			COALESCE(SUM(orders.total_price), 0) as revenue,
			COUNT(orders.id) as rental_count,
			COALESCE(AVG(reviews.rating), 0) as average_rating
		`).
		Joins("LEFT JOIN orders ON orders.shop_id = products.shop_id AND orders.status = 'completed' AND orders.created_at BETWEEN ? AND ?", startTime, endTime).
		Joins("LEFT JOIN reviews ON reviews.product_id = products.id").
		Where("products.shop_id = ?", shop.ID).
		Group("products.id").
		Order("revenue DESC").
		Limit(5). // Ambil 5 produk teratas
		Scan(&topProducts)

	// 5. Kirim semua data dalam satu respons JSON
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
	// 1. Dapatkan userID dari token untuk menemukan toko
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	var shop models.Shop
	if err := h.DB.Select("id").Where("user_id = ?", userID).First(&shop).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "User does not own a shop"})
		return
	}

	// 2. Siapkan query untuk mengambil pesanan
	query := h.DB.Model(&models.Order{}).Where("shop_id = ?", shop.ID)

	// 3. (Opsional) Tambahkan filter berdasarkan status jika ada
	statusFilter := c.Query("status")
	if statusFilter != "" {
		query = query.Where("status = ?", statusFilter)
	}

	// 4. Ambil semua pesanan yang cocok dan urutkan berdasarkan yang terbaru
	var orders []models.Order
	if err := query.Order("created_at DESC").Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve orders"})
		return
	}

	// Jika tidak ada pesanan, kembalikan array kosong
	if orders == nil {
		orders = make([]models.Order, 0)
	}

	c.JSON(http.StatusOK, orders)
}