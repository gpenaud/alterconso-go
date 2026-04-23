package service

import (
	"errors"
	"math"
	"time"

	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/gorm"
)

// PaymentType représente un mode de paiement accepté.
type PaymentType struct {
	Type       string `json:"type"`
	Name       string `json:"name"`
	OnTheSpot  bool   `json:"onTheSpot"`
}

// Modes de paiement disponibles.
var allPaymentTypes = []PaymentType{
	{Type: "cash",           Name: "Espèces"},
	{Type: "check",          Name: "Chèque"},
	{Type: "transfer",       Name: "Virement"},
	{Type: "moneypot",       Name: "Cagnotte"},
	{Type: "onthespot",      Name: "Sur place",      OnTheSpot: true},
	{Type: "cardterminal",   Name: "Carte (TPE)",    OnTheSpot: true},
}

type OperationType string

const (
	OpTypeVOrder    OperationType = "VOrder"    // commande variable (dette)
	OpTypeCOrder    OperationType = "COrder"    // commande AMAP fixe (dette)
	OpTypePayment   OperationType = "Payment"   // paiement
	OpTypeMembership OperationType = "Membership" // adhésion
)

type PaymentService struct {
	db *gorm.DB
}

func NewPaymentService(db *gorm.DB) *PaymentService {
	return &PaymentService{db: db}
}

// GetPaymentTypes retourne les types de paiement disponibles pour un groupe.
func (s *PaymentService) GetPaymentTypes(groupID uint) []PaymentType {
	var group model.Group
	if err := s.db.First(&group, groupID).Error; err != nil {
		return nil
	}
	if group.AllowedPaymentsType == nil {
		return nil
	}
	// Parser la liste JSON des types autorisés
	// AllowedPaymentsType est stocké comme CSV simple : "cash,check,transfer"
	var out []PaymentType
	for _, pt := range allPaymentTypes {
		if containsStr(*group.AllowedPaymentsType, pt.Type) {
			out = append(out, pt)
		}
	}
	return out
}

// MakePaymentOperation enregistre un paiement et met à jour la balance.
func (s *PaymentService) MakePaymentOperation(userID, groupID uint, paymentType string, amount float64, name string, relatedOpID *uint) (*model.Operation, error) {
	op := &model.Operation{
		UserID:      userID,
		GroupID:     groupID,
		Amount:      math.Abs(amount),
		Type:        string(OpTypePayment),
		Description: &name,
	}
	if relatedOpID != nil {
		op.RelatedOpID = relatedOpID
	}
	op.PaymentType = &paymentType
	op.Pending = true

	if err := s.db.Create(op).Error; err != nil {
		return nil, err
	}

	s.UpdateUserBalance(userID, groupID)
	return op, nil
}

// MakeOrderOperation crée l'opération de dette associée à des commandes.
func (s *PaymentService) MakeOrderOperation(orders []model.UserOrder, basketID *uint) (*model.Operation, error) {
	if len(orders) == 0 {
		return nil, errors.New("no orders")
	}

	var amount float64
	for _, o := range orders {
		a := o.Quantity * o.ProductPrice
		amount += a + a*(o.FeesRate/100)
	}

	var product model.Product
	s.db.Preload("Catalog.Group").First(&product, orders[0].ProductID)

	op := &model.Operation{
		UserID:  orders[0].UserID,
		GroupID: product.Catalog.GroupID,
		Amount:  -amount, // dette = négatif
		Pending: true,
	}

	if product.Catalog.Type == model.CatalogTypeConstOrder {
		op.Type = string(OpTypeCOrder)
		name := product.Catalog.Name
		op.Description = &name
	} else {
		if basketID == nil {
			return nil, errors.New("basket required for variable orders")
		}
		op.Type = string(OpTypeVOrder)
		op.BasketID = basketID
		name := "Commande du " + time.Now().Format("02/01/2006")
		op.Description = &name
	}

	if err := s.db.Create(op).Error; err != nil {
		return nil, err
	}

	s.UpdateUserBalance(op.UserID, op.GroupID)
	return op, nil
}

// UpdateOrderOperation met à jour une opération de commande existante.
func (s *PaymentService) UpdateOrderOperation(op *model.Operation, orders []model.UserOrder) error {
	var amount float64
	for _, o := range orders {
		a := o.Quantity * o.ProductPrice
		amount += a + a*(o.FeesRate/100)
	}
	op.Amount = -amount
	if err := s.db.Save(op).Error; err != nil {
		return err
	}
	s.UpdateUserBalance(op.UserID, op.GroupID)
	return nil
}

// UpdateUserBalance recalcule et met à jour la balance d'un membre dans un groupe.
// Reproduit : SELECT SUM(amount) FROM Operation WHERE userId=X AND groupId=Y AND NOT (type=Payment AND pending=1)
func (s *PaymentService) UpdateUserBalance(userID, groupID uint) {
	var balance float64
	s.db.Model(&model.Operation{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("user_id = ? AND group_id = ? AND NOT (type = ? AND pending = 1)", userID, groupID, string(OpTypePayment)).
		Scan(&balance)

	balance = math.Round(balance*100) / 100

	var ug model.UserGroup
	if err := s.db.Where("user_id = ? AND group_id = ?", userID, groupID).First(&ug).Error; err != nil {
		return
	}
	s.db.Model(&ug).Update("balance", balance)
}

// ValidateDistribution valide une distribution et marque toutes ses commandes comme payées.
func (s *PaymentService) ValidateDistribution(multiDistribID uint) error {
	var md model.MultiDistrib
	if err := s.db.First(&md, multiDistribID).Error; err != nil {
		return errors.New("distribution not found")
	}
	if md.DistribStartDate.After(time.Now()) {
		return errors.New("cannot validate a distribution that has not started yet")
	}
	if md.Validated {
		return errors.New("distribution already validated")
	}

	// Valider tous les paniers
	var baskets []model.Basket
	s.db.Where("multi_distrib_id = ?", multiDistribID).Find(&baskets)

	for _, basket := range baskets {
		if err := s.validateBasket(basket.ID); err != nil {
			return err
		}
	}

	s.db.Model(&md).Update("validated", true)
	return nil
}

// validateBasket marque les commandes d'un panier comme payées.
func (s *PaymentService) validateBasket(basketID uint) error {
	var orders []model.UserOrder
	s.db.Where("basket_id = ?", basketID).Find(&orders)

	for _, o := range orders {
		s.db.Model(&o).Update("paid", true)
	}

	// Confirmer les opérations en attente
	var op model.Operation
	if err := s.db.Where("basket_id = ? AND type = ?", basketID, string(OpTypeVOrder)).First(&op).Error; err == nil {
		s.db.Model(&op).Updates(map[string]interface{}{"pending": false})
		// Confirmer aussi les paiements liés
		s.db.Model(&model.Operation{}).
			Where("related_op_id = ? AND type = ?", op.ID, string(OpTypePayment)).
			Update("pending", false)

		if len(orders) > 0 {
			s.UpdateUserBalance(orders[0].UserID, op.GroupID)
		}
	}

	return nil
}

// FindVOrderOperation trouve l'opération de commande variable liée à un panier.
func (s *PaymentService) FindVOrderOperation(basketID uint) (*model.Operation, error) {
	var op model.Operation
	err := s.db.Where("basket_id = ? AND type = ?", basketID, string(OpTypeVOrder)).
		Order("created_at DESC").First(&op).Error
	if err != nil {
		return nil, err
	}
	return &op, nil
}

func containsStr(csv, val string) bool {
	for i := 0; i < len(csv); {
		j := i
		for j < len(csv) && csv[j] != ',' {
			j++
		}
		if csv[i:j] == val {
			return true
		}
		i = j + 1
	}
	return false
}
