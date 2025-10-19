// Lokasi: internal/product/handler.go
package product

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strconv"

	"sewascaf.com/api/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Handler struct {
	DB                 *gorm.DB
	SupabaseURL        string
	SupabaseServiceKey string
}

func NewHandler(db *gorm.DB, supabaseURL string, supabaseServiceKey string) *Handler {
	return &Handler{
		DB:                 db,
		SupabaseURL:        supabaseURL,
		SupabaseServiceKey: supabaseServiceKey,
	}
}

func (h *Handler) CreateProduct(c *gin.Context) {
	// 1. Dapatkan userID dari token
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

	// 2. Verifikasi role user adalah 'pengusaha' dan dapatkan tokonya
	var shop models.Shop
	if err := h.DB.Select("id").Where("user_id = ?", userIDString).First(&shop).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "User does not own a shop"})
		return
	}

	// 3. Ambil file gambar dan data teks dari form
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product image is required"})
		return
	}

	// 4. Konversi harga, diskon, dan stok dari string ke integer
	price, err := strconv.Atoi(c.PostForm("price_per_day"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid price format"})
		return
	}

	discountPrice, err := strconv.Atoi(c.PostForm("discount_price_per_day"))
	if err != nil {
		// Jika diskon tidak diisi atau format salah, anggap 0
		discountPrice = 0
	}

	stock, err := strconv.Atoi(c.PostForm("stock"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid stock format"})
		return
	}
	
	// 5. Upload gambar ke Supabase Storage (di bucket 'product-images')
	fileName := fmt.Sprintf("%s-%s", uuid.New().String(), filepath.Base(file.Filename))
	uploadURL := fmt.Sprintf("%s/storage/v1/object/product-images/%s", h.SupabaseURL, fileName)
	
	src, _ := file.Open()
	defer src.Close()

	req, _ := http.NewRequest("POST", uploadURL, src)
	req.Header.Set("Authorization", "Bearer "+h.SupabaseServiceKey)
	req.Header.Set("Content-Type", file.Header.Get("Content-Type"))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Printf("Failed to upload product image: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload product image"})
		return
	}

	// 6. Buat produk baru di database
	newProduct := models.Product{
		ID:                  uuid.New(),
		ShopID:              shop.ID,
		SKU:                 c.PostForm("sku"),
		Name:                c.PostForm("name"),
		Description:         c.PostForm("description"),
		PricePerDay:         price,
		DiscountPricePerDay: discountPrice, // <-- Tambahkan data diskon
		Stock:               stock,
		ImageURL:            fmt.Sprintf("%s/storage/v1/object/public/product-images/%s", h.SupabaseURL, fileName),
	}

	if result := h.DB.Create(&newProduct); result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create product", "details": result.Error.Error()})
		return
	}

	c.JSON(http.StatusCreated, newProduct)
}

func (h *Handler) GetShopProducts(c *gin.Context) {
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

	var products []models.Product
	if err := h.DB.Where("shop_id = ?", shop.ID).Find(&products).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve products"})
		return
	}

	if products == nil {
		products = make([]models.Product, 0)
	}

	c.JSON(http.StatusOK, products)
}

type UpdateProductPayload struct {
	Name                string `json:"name"`
	Description         string `json:"description"`
	PricePerDay         int    `json:"price_per_day"`
	DiscountPricePerDay int    `json:"discount_price_per_day"`
	Stock               int    `json:"stock"`
}

func (h *Handler) UpdateProduct(c *gin.Context) {
	// 1. Dapatkan productID dari URL
	productID := c.Param("productId")

	// 2. Dapatkan userID dari token
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	// 3. Bind data JSON dari request
	var payload UpdateProductPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// 4. Lakukan Transaction untuk keamanan
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		// Langkah A: Cari toko milik user untuk mendapatkan ShopID
		var shop models.Shop
		if err := tx.Select("id").Where("user_id = ?", userID).First(&shop).Error; err != nil {
			return errors.New("shop not found for this user")
		}

		// Langkah B: Cari produk berdasarkan ID-nya DAN pastikan produk itu milik toko si user
		var product models.Product
		if err := tx.Where("id = ? AND shop_id = ?", productID, shop.ID).First(&product).Error; err != nil {
			return errors.New("product not found or you do not have permission to edit it")
		}

		// Langkah C: Lakukan update
		if err := tx.Model(&product).Updates(payload).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		if err.Error() == "product not found or you do not have permission to edit it" {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Product updated successfully"})
}

func (h *Handler) DeleteProduct(c *gin.Context) {
	productID := c.Param("productId")

	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		var shop models.Shop
		if err := tx.Select("id").Where("user_id = ?", userID).First(&shop).Error; err != nil {
			return errors.New("shop not found for this user")
		}

		var product models.Product
		if err := tx.Where("id = ? AND shop_id = ?", productID, shop.ID).First(&product).Error; err != nil {
			return errors.New("product not found or you do not have permission to delete it")
		}

		if err := tx.Delete(&product).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		if err.Error() == "product not found or you do not have permission to delete it" {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Product succesfully deletted"})
}

type CreateReviewPayload struct {
	Rating  int    `json:"rating" binding:"required,min=1,max=5"`
	Comment string `json:"comment" binding:"required"`
}

func (h *Handler) CreateReview(c *gin.Context) {
	productID := c.Param("productId")
	userID, _ := c.Get("userID")

	var payload CreateReviewPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body, rating must be between 1 and 5"})
		return
	}

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		// Validasi 1: Cek apakah user pernah menyelesaikan pesanan untuk produk ini
		var count int64
		tx.Model(&models.OrderItem{}).
			Joins("JOIN orders ON orders.id = order_items.order_id").
			Where("order_items.product_id = ? AND orders.user_id = ? AND orders.status = ?", productID, userID, "completed").
			Count(&count)

		if count == 0 {
			return errors.New("you can only review products you have completed renting")
		}

		// Validasi 2: Cek apakah user sudah pernah memberikan ulasan untuk produk ini
		tx.Model(&models.Review{}).
			Where("product_id = ? AND user_id = ?", productID, userID).
			Count(&count)
		
		if count > 0 {
			return errors.New("you have already reviewed this product")
		}

		// Jika lolos validasi, buat ulasan baru
		newReview := models.Review{
			ID:        uuid.New(),
			UserID:    uuid.MustParse(userID.(string)),
			ProductID: uuid.MustParse(productID),
			Rating:    payload.Rating,
			Comment:   payload.Comment,
		}

		if err := tx.Create(&newReview).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Review submitted successfully"})
}