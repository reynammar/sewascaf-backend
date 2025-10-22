package models

import (
	"time"

	"database/sql/driver"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
)

type JSONB []string
func (j JSONB) Value() (driver.Value, error) {
    return json.Marshal(j)
}
func (j *JSONB) Scan(src interface{}) error {
    source, ok := src.([]byte)
    if !ok {
        return errors.New("type assertion .([]byte) failed")
    }
    return json.Unmarshal(source, &j)
}

type User struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;"`
	Name      string    `json:"name"`
	Email     string    `json:"email" gorm:"unique"`
	Password  string    `json:"-"`
	Pekerjaan string    `json:"pekerjaan"`
	Alamat    string    `json:"alamat"`
	Telepon   string    `json:"telepon"`
	Role      string    `json:"role"`
}

type Shop struct {
	ID                  uuid.UUID `json:"id" gorm:"type:uuid;primary_key;"`
	UserID              uuid.UUID `json:"-" gorm:"type:uuid"`
	User                User      `json:"-" gorm:"foreignKey:UserID"`
	ShopName            string    `json:"shop_name"`
	ShopAddress         string    `json:"shop_address"`
	ShopPhoneNumber     string    `json:"shop_phone_number"`
	ShopDescription     string    `json:"shop_description"`
	ShopProfileImageURL string    `json:"shop_profile_image_url"`
	ShopNameLastUpdated *time.Time `json:"shop_name_last_updated"`
	ActivePaymentChannels JSONB `json:"active_payment_channels" gorm:"type:jsonb"`
}

type Product struct {
	ID                  uuid.UUID `json:"id" gorm:"type:uuid;primary_key;"`
	ShopID              uuid.UUID `json:"shop_id" gorm:"type:uuid"`
	Shop                Shop      `json:"shop" gorm:"foreignKey:ShopID"`
	SKU                 string    `json:"sku" gorm:"unique"`
	Name                string    `json:"name"`
	Category            string    `json:"category"`
	Description         string    `json:"description"`
	PricePerDay         int       `json:"price_per_day"`         
	DiscountPricePerDay int       `json:"discount_price_per_day"`  
	Stock               int       `json:"stock"`                 
	ImageURL            string    `json:"image_url"`
	Reviews             []Review  `json:"reviews" gorm:"foreignKey:ProductID"`
}

type Order struct {
	ID         uuid.UUID `json:"id" gorm:"type:uuid;primary_key;"`
	UserID     uuid.UUID `json:"user_id" gorm:"type:uuid"`    
	User       User      `json:"-" gorm:"foreignKey:UserID"`
	ShopID     uuid.UUID `json:"shop_id" gorm:"type:uuid"`    
	Shop       Shop        `json:"shop" gorm:"foreignKey:ShopID"`
	TotalPrice int       `json:"total_price"`
	Status     string    `json:"status"`
	StartDate  time.Time `json:"start_date"`
	EndDate    time.Time `json:"end_date"`
	CreatedAt  time.Time `json:"created_at"`
	OrderItems []OrderItem `json:"items" gorm:"foreignKey:OrderID"`
	PaymentMethod string    `json:"payment_method"`
}

type Review struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;"`
	UserID    uuid.UUID `json:"user_id" gorm:"type:uuid"`   
	User      User      `json:"-" gorm:"foreignKey:UserID"`
	ProductID uuid.UUID `json:"product_id" gorm:"type:uuid"`
	Product   Product   `json:"-" gorm:"foreignKey:ProductID"`
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment"`
}

type OrderItem struct {
	ID                 uuid.UUID `json:"id" gorm:"type:uuid;primary_key;"`
	OrderID            uuid.UUID `json:"order_id" gorm:"type:uuid"`
	Order              Order     `json:"-" gorm:"foreignKey:OrderID"`
	ProductID          uuid.UUID `json:"product_id" gorm:"type:uuid"`
	Product            Product   `json:"-" gorm:"foreignKey:ProductID"`
	Quantity           int       `json:"quantity"`
	PriceAtTimeOfOrder int       `json:"price_at_time_of_order"` 
}

type Bookmark struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;"`
	UserID    uuid.UUID `json:"user_id" gorm:"type:uuid;uniqueIndex:idx_user_product"`
	User      User      `json:"-" gorm:"foreignKey:UserID"`
	ProductID uuid.UUID `json:"product_id" gorm:"type:uuid;uniqueIndex:idx_user_product"`
	Product   Product   `json:"-" gorm:"foreignKey:ProductID"`
}

type ChatHistory struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;"`
	UserID    uuid.UUID `json:"user_id" gorm:"type:uuid;index"`
	User      User      `json:"-" gorm:"foreignKey:UserID"`
	Question  string    `json:"question"`
	Answer    string    `json:"answer"`
	CreatedAt time.Time `json:"created_at"`
}