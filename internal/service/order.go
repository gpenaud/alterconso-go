package service

import (
	"errors"
	"math"

	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/gorm"
)

type OrderService struct {
	db *gorm.DB
}

func NewOrderService(db *gorm.DB) *OrderService {
	return &OrderService{db: db}
}

// canHaveFloatQt retourne true si le produit accepte des quantités décimales.
func canHaveFloatQt(p *model.Product) bool {
	return p.UnitType == model.UnitTypeKilogram ||
		p.UnitType == model.UnitTypeGram ||
		p.UnitType == model.UnitTypeLitre ||
		p.UnitType == model.UnitTypeCentilitre ||
		p.UnitType == model.UnitTypeMillilitre
}

// isInt retourne true si f est un entier (ex: 2.0 oui, 2.5 non).
func isInt(f float64) bool {
	return math.Mod(f, 1.0) == 0
}

// MakeOrderInput contient les données pour créer une commande.
type MakeOrderInput struct {
	User           *model.User
	Product        *model.Product
	DistributionID uint
	Quantity       float64
	Paid           bool
	User2ID        *uint
	InvertShared   bool
	SubscriptionID *uint
}

// Make crée ou cumule une commande pour un utilisateur.
// Reproduit la logique de OrderService.make() en Haxe.
func (s *OrderService) Make(in MakeOrderInput) (*model.UserOrder, error) {
	if in.DistributionID == 0 {
		return nil, errors.New("distribId is required")
	}
	if in.Quantity < 0 {
		return nil, errors.New("quantity cannot be negative")
	}
	if in.Quantity == 0 {
		return nil, nil
	}

	// Charger le produit avec son catalogue
	var product model.Product
	if err := s.db.Preload("Catalog").First(&product, in.Product.ID).Error; err != nil {
		return nil, errors.New("product not found")
	}

	// Vérification des quantités décimales
	if !canHaveFloatQt(&product) && !isInt(in.Quantity) {
		return nil, errors.New("product quantity must be an integer")
	}

	// Charger la distribution
	var distrib model.Distribution
	if err := s.db.Preload("Catalog").Preload("MultiDistrib").First(&distrib, in.DistributionID).Error; err != nil {
		return nil, errors.New("distribution not found")
	}

	// Chercher une commande existante à cumuler
	var prevOrders []model.UserOrder
	s.db.Where("product_id = ? AND user_id = ? AND distribution_id = ?",
		product.ID, in.User.ID, in.DistributionID).Find(&prevOrders)

	order := &model.UserOrder{
		UserID:       in.User.ID,
		ProductID:    product.ID,
		ProductPrice: product.Price,
		Quantity:     in.Quantity,
		Paid:         in.Paid,
	}

	// Frais en pourcentage
	if product.Catalog.HasFlag(model.CatalogFlagHasPercentageFees) && product.Catalog.PercentageFees != nil {
		order.FeesRate = *product.Catalog.PercentageFees
	}

	// Commande partagée
	if in.User2ID != nil {
		order.User2ID = in.User2ID
		if in.InvertShared {
			order.Flags = uint(model.OrderFlagInvertSharedOrder)
		}
	}

	distID := in.DistributionID
	order.DistributionID = &distID

	// Trouver ou créer le panier
	basket, err := s.getOrCreateBasket(in.User.ID, distrib.MultiDistribID)
	if err != nil {
		return nil, err
	}
	order.BasketID = &basket.ID

	if in.SubscriptionID != nil {
		order.SubscriptionID = in.SubscriptionID
	}

	// Cumuler les quantités si commande précédente existante
	for _, prev := range prevOrders {
		order.Quantity += prev.Quantity
		s.db.Delete(&prev)
	}

	// Gestion des stocks
	finalQty, err := s.applyStock(&product, order.Quantity)
	if err != nil {
		return nil, err
	}
	if finalQty <= 0 {
		return nil, nil // stock épuisé
	}
	order.Quantity = finalQty

	if err := s.db.Create(order).Error; err != nil {
		return nil, err
	}

	return order, nil
}

// Edit modifie la quantité d'une commande existante.
// Reproduit la logique de OrderService.edit() en Haxe.
func (s *OrderService) Edit(order *model.UserOrder, newQty float64, paid *bool, user2ID *uint, invert *bool) (*model.UserOrder, error) {
	if newQty < 0 {
		return nil, errors.New("quantity cannot be negative")
	}

	var product model.Product
	if err := s.db.Preload("Catalog").First(&product, order.ProductID).Error; err != nil {
		return nil, errors.New("product not found")
	}

	if !canHaveFloatQt(&product) && !isInt(newQty) {
		return nil, errors.New("product quantity must be an integer")
	}

	// Gestion du paiement
	if paid != nil {
		order.Paid = *paid
	} else if newQty > order.Quantity {
		order.Paid = false
	}

	// Commande partagée
	if user2ID != nil {
		order.User2ID = user2ID
		if invert != nil {
			if *invert {
				order.Flags = uint(model.OrderFlagInvertSharedOrder)
			} else {
				order.Flags = 0
			}
		}
	} else {
		order.User2ID = nil
		order.Flags = 0
	}

	// Gestion des stocks
	if product.Stock != nil && product.Catalog.HasFlag(model.CatalogFlagStockManagement) {
		diff := newQty - order.Quantity
		newStock := *product.Stock - diff

		if diff > 0 && newStock < 0 {
			// Stock insuffisant : limiter la quantité
			newQty = order.Quantity + *product.Stock
			zero := 0.0
			product.Stock = &zero
		} else {
			newStock = *product.Stock - diff
			product.Stock = &newStock
		}
		s.db.Model(&product).Update("stock", product.Stock)
	}

	order.Quantity = newQty
	if newQty == 0 {
		order.Paid = true
	}

	if err := s.db.Save(order).Error; err != nil {
		return nil, err
	}

	return order, nil
}

// Delete supprime une commande (uniquement si quantité == 0).
func (s *OrderService) Delete(order *model.UserOrder) error {
	if order.Quantity != 0 {
		return errors.New("cannot delete order with non-zero quantity")
	}

	// Restaurer le stock si nécessaire
	var product model.Product
	if err := s.db.Preload("Catalog").First(&product, order.ProductID).Error; err == nil {
		if product.Stock != nil && product.Catalog.HasFlag(model.CatalogFlagStockManagement) {
			newStock := *product.Stock + order.Quantity
			product.Stock = &newStock
			s.db.Model(&product).Update("stock", newStock)
		}
	}

	return s.db.Delete(order).Error
}

// GetUserOrders retourne les commandes d'un utilisateur pour une distribution ou un catalogue.
func (s *OrderService) GetUserOrders(userID, distributionID uint, catalogID *uint) ([]model.UserOrder, error) {
	q := s.db.Where("user_id = ? AND distribution_id = ?", userID, distributionID).
		Preload("Product.Catalog").
		Preload("User").
		Preload("User2")

	if catalogID != nil {
		q = q.Joins("JOIN products ON products.id = user_orders.product_id").
			Where("products.catalog_id = ?", *catalogID)
	}

	var orders []model.UserOrder
	if err := q.Find(&orders).Error; err != nil {
		return nil, err
	}
	return orders, nil
}

// CreateOrUpdateOrders applique un tableau de commandes (upsert).
// Reproduit la logique de OrderService.createOrUpdateOrders() en Haxe.
type OrderData struct {
	ID        *uint   `json:"id"`
	ProductID uint    `json:"productId"`
	Quantity  float64 `json:"qt"`
	Paid      bool    `json:"paid"`
	User2ID   *uint   `json:"userId2"`
	Invert    bool    `json:"invertSharedOrder"`
}

func (s *OrderService) CreateOrUpdateOrders(user *model.User, distributionID uint, catalogID *uint, ordersData []OrderData) ([]model.UserOrder, error) {
	if len(ordersData) == 0 {
		return nil, errors.New("no orders provided")
	}

	existing, err := s.GetUserOrders(user.ID, distributionID, catalogID)
	if err != nil {
		return nil, err
	}

	// Index des commandes existantes par ID
	existingByID := make(map[uint]*model.UserOrder)
	for i := range existing {
		existingByID[existing[i].ID] = &existing[i]
	}

	var result []model.UserOrder

	for _, od := range ordersData {
		// Commande existante → modifier
		if od.ID != nil {
			if prev, ok := existingByID[*od.ID]; ok {
				updated, err := s.Edit(prev, od.Quantity, &od.Paid, od.User2ID, &od.Invert)
				if err != nil {
					return nil, err
				}
				if updated != nil {
					result = append(result, *updated)
				}
				continue
			}
		}

		// Nouvelle commande → créer
		var product model.Product
		if err := s.db.First(&product, od.ProductID).Error; err != nil {
			continue
		}
		newOrder, err := s.Make(MakeOrderInput{
			User:           user,
			Product:        &product,
			DistributionID: distributionID,
			Quantity:       od.Quantity,
			Paid:           od.Paid,
			User2ID:        od.User2ID,
			InvertShared:   od.Invert,
		})
		if err != nil {
			return nil, err
		}
		if newOrder != nil {
			result = append(result, *newOrder)
		}
	}

	return result, nil
}

// applyStock décrémente le stock et retourne la quantité finale accordée.
func (s *OrderService) applyStock(product *model.Product, qty float64) (float64, error) {
	if product.Stock == nil || !product.Catalog.HasFlag(model.CatalogFlagStockManagement) {
		return qty, nil
	}

	stock := *product.Stock

	if stock <= 0 {
		return 0, nil
	}

	finalQty := qty
	if stock-qty < 0 {
		finalQty = stock
	}

	newStock := stock - finalQty
	product.Stock = &newStock
	s.db.Model(product).Update("stock", newStock)

	return finalQty, nil
}

// getOrCreateBasket trouve ou crée le panier d'un utilisateur pour un MultiDistrib.
func (s *OrderService) getOrCreateBasket(userID, multiDistribID uint) (*model.Basket, error) {
	var basket model.Basket
	err := s.db.Where("user_id = ? AND multi_distrib_id = ?", userID, multiDistribID).First(&basket).Error
	if err == nil {
		return &basket, nil
	}
	basket = model.Basket{
		UserID:         userID,
		MultiDistribID: multiDistribID,
	}
	if err := s.db.Create(&basket).Error; err != nil {
		return nil, err
	}
	return &basket, nil
}
