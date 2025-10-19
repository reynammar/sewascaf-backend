package models

import (
	"time"

	"github.com/google/uuid"
)

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
}

type Product struct {
	ID                  uuid.UUID `json:"id" gorm:"type:uuid;primary_key;"`
	ShopID              uuid.UUID `json:"shop_id" gorm:"type:uuid"`
	Shop                Shop      `json:"-" gorm:"foreignKey:ShopID"`
	SKU                 string    `json:"sku" gorm:"unique"`
	Name                string    `json:"name"`
	Description         string    `json:"description"`
	PricePerDay         int       `json:"price_per_day"`         
	DiscountPricePerDay int       `json:"discount_price_per_day"`  
	Stock               int       `json:"stock"`                 
	ImageURL            string    `json:"image_url"`
}

type Order struct {
	ID         uuid.UUID `json:"id" gorm:"type:uuid;primary_key;"`
	UserID     uuid.UUID `json:"user_id" gorm:"type:uuid"`    
	User       User      `json:"-" gorm:"foreignKey:UserID"`
	ShopID     uuid.UUID `json:"shop_id" gorm:"type:uuid"`    
	Shop       Shop      `json:"-" gorm:"foreignKey:ShopID"`
	TotalPrice int       `json:"total_price"`
	Status     string    `json:"status"`
	StartDate  time.Time `json:"start_date"`
	EndDate    time.Time `json:"end_date"`
	CreatedAt  time.Time `json:"created_at"`
}

// Review mendefinisikan struktur data untuk setiap ulasan produk
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