package main

import (
	"log"

	"sewascaf.com/api/internal/auth"
	"sewascaf.com/api/internal/bookmark"
	"sewascaf.com/api/internal/chatbot"
	"sewascaf.com/api/internal/config"
	"sewascaf.com/api/internal/database"
	"sewascaf.com/api/internal/middleware"
	"sewascaf.com/api/internal/models"
	"sewascaf.com/api/internal/order"
	"sewascaf.com/api/internal/product"
	"sewascaf.com/api/internal/shop"
	"sewascaf.com/api/internal/tripay"
	"sewascaf.com/api/internal/user"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Could not load config: %v", err)
	}

	db := database.InitDB(cfg.DatabaseURL)

	// runMigrations(db)

	router := gin.Default()

	authHandler := auth.NewHandler(db, cfg.JWTSecret)
	userHandler := user.NewHandler(db, cfg.SupabaseURL, cfg.SupabaseServiceKey, cfg.JWTSecret)
	productHandler := product.NewHandler(db, cfg.SupabaseURL, cfg.SupabaseServiceKey)
	tripayHandler := tripay.NewHandler(db, cfg.TripayAPIKey, cfg.TripayPrivateKey)
	shopHandler := shop.NewHandler(db)
	bookmarkHandler := bookmark.NewHandler(db)
	orderHandler := order.NewHandler(db, cfg.TripayAPIKey, cfg.TripayPrivateKey, cfg.TripayMerchantCode)
	chatbotHandler := chatbot.NewHandler(db, cfg.GeminiAPIKey)

	v1 := router.Group("/api/v1")
	{
		// AI BOT
		v1.POST("/chatbot/ask", middleware.AuthMiddleware(cfg.JWTSecret), chatbotHandler.AskChatbot)
		// User
		v1.GET("/products", productHandler.GetProducts)
		v1.GET("/products/:productId", productHandler.GetProductDetail)

		v1.POST("/products/:productId/bookmarks", middleware.AuthMiddleware(cfg.JWTSecret), bookmarkHandler.AddBookmark)
		v1.DELETE("/products/:productId/bookmarks", middleware.AuthMiddleware(cfg.JWTSecret), bookmarkHandler.DeleteBookmark)
		v1.GET("/users/me/bookmarks", middleware.AuthMiddleware(cfg.JWTSecret), bookmarkHandler.GetUserBookmarks)

		v1.POST("/orders", middleware.AuthMiddleware(cfg.JWTSecret), orderHandler.CreateOrder)
		v1.POST("/tripay/callback", tripayHandler.CallbackHandler)
		v1.GET("/users/me/orders", middleware.AuthMiddleware(cfg.JWTSecret), orderHandler.GetUserOrders)
		v1.POST("/orders/:orderId/cancel", middleware.AuthMiddleware(cfg.JWTSecret), orderHandler.CancelOrder)

		// Auth
		v1.POST("/register", authHandler.Register)
		v1.POST("/login", authHandler.Login)
		v1.GET("/users/profile", middleware.AuthMiddleware(cfg.JWTSecret), userHandler.GetProfile)
		v1.POST("/users/upgrade-to-vendor", middleware.AuthMiddleware(cfg.JWTSecret), userHandler.UpgradeToVendor)

		// Vendor

		v1.GET("/shops/me", middleware.AuthMiddleware(cfg.JWTSecret), shopHandler.GetShopProfile)
		v1.PUT("/shops/me", middleware.AuthMiddleware(cfg.JWTSecret), shopHandler.UpdateShopProfile)
		v1.GET("/shops/me/orders", middleware.AuthMiddleware(cfg.JWTSecret), shopHandler.GetShopOrders)
		v1.PUT("/orders/:orderId/status", middleware.AuthMiddleware(cfg.JWTSecret), shopHandler.UpdateOrderStatus)
		
		v1.POST("/products", middleware.AuthMiddleware(cfg.JWTSecret), productHandler.CreateProduct)
		v1.GET("/products/my-shop", middleware.AuthMiddleware(cfg.JWTSecret), productHandler.GetShopProducts)
		v1.PUT("/products/:productId", middleware.AuthMiddleware(cfg.JWTSecret), productHandler.UpdateProduct)
		v1.DELETE("/products/:productId", middleware.AuthMiddleware(cfg.JWTSecret), productHandler.DeleteProduct)

		v1.GET("/shops/me/statistics", middleware.AuthMiddleware(cfg.JWTSecret), shopHandler.GetShopStatistics)
		v1.POST("/products/:productId/reviews", middleware.AuthMiddleware(cfg.JWTSecret), productHandler.CreateReview)

		v1.GET("/payment-channels", middleware.AuthMiddleware(cfg.JWTSecret), tripayHandler.GetPaymentChannels)
		v1.PUT("/shops/me/payment-channels", middleware.AuthMiddleware(cfg.JWTSecret), shopHandler.UpdatePaymentChannels)
		v1.GET("/shops/me/payment-channels", middleware.AuthMiddleware(cfg.JWTSecret), shopHandler.GetShopPaymentChannels)
	}

	router.Run(":8080")
}

func runMigrations(db *gorm.DB) {
	log.Println("Running database migrations...")
	err := db.AutoMigrate(&models.User{}, &models.Shop{}, &models.Product{}, &models.Order{}, &models.Review{}, &models.OrderItem{}, &models.Bookmark{}, &models.ChatHistory{})
	if err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}
	log.Println("✅ Database migrated successfully.")
}