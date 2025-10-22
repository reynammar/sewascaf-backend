package order

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"sewascaf.com/api/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Handler struct {
	DB                 *gorm.DB
	TripayAPIKey       string
	TripayPrivateKey   string
	TripayMerchantCode string
}

func NewHandler(db *gorm.DB, apiKey, privateKey, merchantCode string) *Handler {
	return &Handler{
		DB:                 db,
		TripayAPIKey:       apiKey,
		TripayPrivateKey:   privateKey,
		TripayMerchantCode: merchantCode,
	}
}

type OrderItemPayload struct {
	ProductID string `json:"product_id" binding:"required"`
	Quantity  int    `json:"quantity" binding:"required,gt=0"`
}

type CreateOrderPayload struct {
	ShopID        string             `json:"shop_id" binding:"required"`
	StartDate     string             `json:"start_date" binding:"required"`
	EndDate       string             `json:"end_date" binding:"required"`
	PaymentMethod string             `json:"payment_method" binding:"required"`
	Items         []OrderItemPayload `json:"items" binding:"required,min=1"`
}

type TripayResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func (h *Handler) CreateOrder(c *gin.Context) {
	userIDInterface, _ := c.Get("userID")
	userIDString, _ := userIDInterface.(string)

	var payload CreateOrderPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	startDate, err := time.Parse("2006-01-02", payload.StartDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_date format, use YYYY-MM-DD"})
		return
	}
	endDate, err := time.Parse("2006-01-02", payload.EndDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_date format, use YYYY-MM-DD"})
		return
	}

	var totalOrderPrice int = 0
	var newOrderItems []models.OrderItem
	var orderProducts []map[string]interface{}
	var newOrder models.Order

	err = h.DB.Transaction(func(tx *gorm.DB) error {
		for _, item := range payload.Items {
			var product models.Product
			if err := tx.First(&product, "id = ?", item.ProductID).Error; err != nil {
				return errors.New("product with id " + item.ProductID + " not found")
			}
			var rentedQuantity int64
			tx.Model(&models.OrderItem{}).Joins("JOIN orders ON orders.id = order_items.order_id").Where(`order_items.product_id = ? AND orders.status IN ('active', 'pending') AND (orders.start_date, orders.end_date) OVERLAPS (?, ?)`, item.ProductID, startDate, endDate).Select("COALESCE(SUM(order_items.quantity), 0)").Scan(&rentedQuantity)
			availableStock := int64(product.Stock) - rentedQuantity
			if availableStock < int64(item.Quantity) {
				return errors.New("stock for product " + product.Name + " is not available on the selected dates")
			}
			var effectivePrice int
			if product.DiscountPricePerDay > 0 {
				effectivePrice = product.DiscountPricePerDay
			} else {
				effectivePrice = product.PricePerDay
			}
			durationDays := int(endDate.Sub(startDate).Hours() / 24)
			if durationDays < 1 {
				durationDays = 1
			}
			itemTotalPriceForDuration := effectivePrice * durationDays
			subTotal := item.Quantity * itemTotalPriceForDuration
			totalOrderPrice += subTotal
			newOrderItems = append(newOrderItems, models.OrderItem{ID: uuid.New(), ProductID: product.ID, Quantity: item.Quantity, PriceAtTimeOfOrder: effectivePrice})
			orderProducts = append(orderProducts, map[string]interface{}{"sku": product.SKU, "name": product.Name, "price": itemTotalPriceForDuration, "quantity": item.Quantity})
		}

		newOrder = models.Order{
			ID:            uuid.New(),
			UserID:        uuid.MustParse(userIDString),
			ShopID:        uuid.MustParse(payload.ShopID),
			TotalPrice:    totalOrderPrice,
			Status:        "pending",
			StartDate:     startDate,
			EndDate:       endDate,
			PaymentMethod: payload.PaymentMethod,
		}

		if err := tx.Create(&newOrder).Error; err != nil {
			return err
		}
		for i := range newOrderItems {
			newOrderItems[i].OrderID = newOrder.ID
		}
		if err := tx.Create(&newOrderItems).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	h.DB.First(&user, "id = ?", userIDString)
	merchantRef := newOrder.ID.String()
	amount := newOrder.TotalPrice
	mac := hmac.New(sha256.New, []byte(h.TripayPrivateKey))
	mac.Write([]byte(h.TripayMerchantCode + merchantRef + strconv.Itoa(amount)))
	signature := hex.EncodeToString(mac.Sum(nil))

	tripayPayload := map[string]interface{}{
		"method":         payload.PaymentMethod,
		"merchant_ref":   merchantRef,
		"amount":         amount,
		"customer_name":  user.Name,
		"customer_email": user.Email,
		"order_items":    orderProducts,
		"signature":      signature,
	}

	payloadBytes, _ := json.Marshal(tripayPayload)
	req, _ := http.NewRequest("POST", "https://tripay.co.id/api-sandbox/transaction/create", strings.NewReader(string(payloadBytes)))
	req.Header.Set("Authorization", "Bearer "+h.TripayAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create transaction with Tripay"})
		return
	}
	defer resp.Body.Close()

	var tripayResponse TripayResponse
	json.NewDecoder(resp.Body).Decode(&tripayResponse)
	if !tripayResponse.Success {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tripay returned an error", "details": tripayResponse.Message})
		return
	}

	c.JSON(http.StatusCreated, tripayResponse.Data)
}

type ShopSummaryForOrder struct {
	ShopName            string `json:"shop_name"`
	ShopAddress         string `json:"shop_address"`
	ShopPhoneNumber     string `json:"shop_phone_number"`
	ShopProfileImageURL string `json:"shop_profile_image_url"`
}
type ProductSummaryForOrder struct {
	Name     string `json:"name"`
	ImageURL string `json:"image_url"`
}
type OrderItemForHistory struct {
	Product  ProductSummaryForOrder `json:"product"`
	Quantity int                    `json:"quantity"`
}
type OrderHistoryResponse struct {
	ID            uuid.UUID             `json:"id"`
	Shop          ShopSummaryForOrder   `json:"shop"`
	TotalPrice    int                   `json:"total_price"`
	Status        string                `json:"status"`
	StartDate     time.Time             `json:"start_date"`
	EndDate       time.Time             `json:"end_date"`
	CreatedAt     time.Time             `json:"created_at"`
	PaymentMethod string                `json:"payment_method"`
	Items         []OrderItemForHistory `json:"items"`
}

// REVISI TOTAL: Ganti Preload dengan JOIN Manual
type FlatOrderHistoryItem struct {
	OrderID               uuid.UUID
	TotalPrice            int
	Status                string
	StartDate             time.Time
	EndDate               time.Time
	CreatedAt             time.Time
	PaymentMethod         string
	ShopName              string
	ShopAddress           string
	ShopPhoneNumber       string
	ShopProfileImageURL   string
	ProductName           string
	ProductImageURL       string
	Quantity              int
}

func (h *Handler) GetUserOrders(c *gin.Context) {
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

	var flatResults []FlatOrderHistoryItem
	err := h.DB.Table("orders").
		Select(`
			orders.id as order_id, orders.total_price, orders.status, orders.start_date, orders.end_date, orders.created_at, orders.payment_method,
			shops.shop_name, shops.shop_address, shops.shop_phone_number, shops.shop_profile_image_url,
			products.name as product_name, products.image_url as product_image_url,
			order_items.quantity
		`).
		Joins("JOIN shops ON shops.id = orders.shop_id").
		Joins("JOIN order_items ON order_items.order_id = orders.id").
		Joins("JOIN products ON products.id = order_items.product_id").
		Where("orders.user_id = ?", userIDString).
		Order("orders.created_at DESC").
		Scan(&flatResults).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve user orders", "details": err.Error()})
		return
	}

	orderMap := make(map[uuid.UUID]*OrderHistoryResponse)
	for _, item := range flatResults {
		if _, exists := orderMap[item.OrderID]; !exists {
			orderMap[item.OrderID] = &OrderHistoryResponse{
				ID: item.OrderID,
				Shop: ShopSummaryForOrder{
					ShopName:            item.ShopName,
					ShopAddress:         item.ShopAddress,
					ShopPhoneNumber:     item.ShopPhoneNumber,
					ShopProfileImageURL: item.ShopProfileImageURL,
				},
				TotalPrice:    item.TotalPrice,
				Status:        item.Status,
				StartDate:     item.StartDate,
				EndDate:       item.EndDate,
				CreatedAt:     item.CreatedAt,
				PaymentMethod: item.PaymentMethod,
				Items:         make([]OrderItemForHistory, 0),
			}
		}
		orderMap[item.OrderID].Items = append(orderMap[item.OrderID].Items, OrderItemForHistory{
			Product: ProductSummaryForOrder{
				Name:     item.ProductName,
				ImageURL: item.ProductImageURL,
			},
			Quantity: item.Quantity,
		})
	}
	
	var finalResponse []OrderHistoryResponse
	processedOrders := make(map[uuid.UUID]bool)
	for _, item := range flatResults {
		if !processedOrders[item.OrderID] {
			finalResponse = append(finalResponse, *orderMap[item.OrderID])
			processedOrders[item.OrderID] = true
		}
	}

	if finalResponse == nil {
		finalResponse = make([]OrderHistoryResponse, 0)
	}

	c.JSON(http.StatusOK, finalResponse)
}

func (h *Handler) CancelOrder(c *gin.Context) {
	userIDInterface, _ := c.Get("userID")
	userIDString, _ := userIDInterface.(string)
	orderID := c.Param("orderId")

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		var order models.Order
		if err := tx.Where("id = ? AND user_id = ?", orderID, userIDString).First(&order).Error; err != nil {
			return errors.New("order not found or you do not have permission to cancel it")
		}

		if order.Status != "pending" {
			return errors.New("only pending orders can be cancelled")
		}

		if err := tx.Model(&order).Update("status", "cancelled").Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Order has been successfully cancelled"})
}