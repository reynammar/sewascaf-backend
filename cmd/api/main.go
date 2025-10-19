package main

import (
	"log"

	"sewascaf.com/api/internal/auth"
	"sewascaf.com/api/internal/config"
	"sewascaf.com/api/internal/database"
	"sewascaf.com/api/internal/middleware"
	"sewascaf.com/api/internal/models"
	"sewascaf.com/api/internal/product"
	"sewascaf.com/api/internal/shop"
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

	runMigrations(db)

	router := gin.Default()

	authHandler := auth.NewHandler(db, cfg.JWTSecret)
	userHandler := user.NewHandler(db, cfg.SupabaseURL, cfg.SupabaseServiceKey, cfg.JWTSecret)
	productHandler := product.NewHandler(db, cfg.SupabaseURL, cfg.SupabaseServiceKey)
	shopHandler := shop.NewHandler(db)

	v1 := router.Group("/api/v1")
	{
		v1.POST("/register", authHandler.Register)
		v1.POST("/login", authHandler.Login)
		v1.GET("/users/profile", middleware.AuthMiddleware(cfg.JWTSecret), userHandler.GetProfile)
		v1.POST("/users/upgrade-to-vendor", middleware.AuthMiddleware(cfg.JWTSecret), userHandler.UpgradeToVendor)

		v1.GET("/shops/me", middleware.AuthMiddleware(cfg.JWTSecret), shopHandler.GetShopProfile)
		v1.PUT("/shops/me", middleware.AuthMiddleware(cfg.JWTSecret), shopHandler.UpdateShopProfile)
		v1.GET("/shops/me/orders", middleware.AuthMiddleware(cfg.JWTSecret), shopHandler.GetShopOrders)
		
		v1.POST("/products", middleware.AuthMiddleware(cfg.JWTSecret), productHandler.CreateProduct)
		v1.GET("/products/my-shop", middleware.AuthMiddleware(cfg.JWTSecret), productHandler.GetShopProducts)
		v1.PUT("/products/:productId", middleware.AuthMiddleware(cfg.JWTSecret), productHandler.UpdateProduct)
		v1.DELETE("/products/:productId", middleware.AuthMiddleware(cfg.JWTSecret), productHandler.DeleteProduct)

		v1.GET("/shops/me/statistics", middleware.AuthMiddleware(cfg.JWTSecret), shopHandler.GetShopStatistics)
		v1.POST("/products/:productId/reviews", middleware.AuthMiddleware(cfg.JWTSecret), productHandler.CreateReview)

	}

	router.Run(":8080")
}

func runMigrations(db *gorm.DB) {
	log.Println("Running database migrations...")
	err := db.AutoMigrate(&models.User{}, &models.Shop{}, &models.Product{}, &models.Order{}, &models.Review{}, &models.OrderItem{})
	if err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}
	log.Println("✅ Database migrated successfully.")
}