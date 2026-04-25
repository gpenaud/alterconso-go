package model

import (
	"crypto/md5"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// UserFlag représente les options de notification d'un utilisateur.
type UserFlag uint

const (
	UserFlagEmailNotif4h        UserFlag = 1 << iota // notification 4h avant distribution
	UserFlagEmailNotif24h                            // notification 24h avant distribution
	UserFlagEmailNotifOuverture                      // notification à l'ouverture des commandes
)

// User correspond à la table `users`.
type User struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `json:"cdate"`
	UpdatedAt time.Time `json:"-"`

	// Identité principale
	FirstName string  `gorm:"size:32;not null"        json:"firstName"`
	LastName  string  `gorm:"size:32;not null"        json:"lastName"`
	Email     string  `gorm:"size:64;uniqueIndex;not null" json:"email"`
	Phone     *string `gorm:"size:19"                 json:"phone,omitempty"`

	// Conjoint / compte partagé
	FirstName2 *string `gorm:"size:32" json:"firstName2,omitempty"`
	LastName2  *string `gorm:"size:32" json:"lastName2,omitempty"`
	Email2     *string `gorm:"size:64;index" json:"email2,omitempty"`
	Phone2     *string `gorm:"size:19"       json:"phone2,omitempty"`

	// Adresse
	Address1 *string `gorm:"size:64" json:"address1,omitempty"`
	Address2 *string `gorm:"size:64" json:"address2,omitempty"`
	ZipCode  *string `gorm:"size:32" json:"zipCode,omitempty"`
	City     *string `gorm:"size:25" json:"city,omitempty"`

	// Identité légale
	BirthDate           *time.Time `json:"birthDate,omitempty"`
	Nationality         *string    `gorm:"size:2" json:"nationality,omitempty"`
	CountryOfResidence  *string    `gorm:"size:2" json:"countryOfResidence,omitempty"`

	// Sécurité
	Pass   string  `gorm:"size:255;not null;default:''" json:"-"`
	APIKey *string `gorm:"size:128;uniqueIndex"         json:"-"`

	// Droits site-wide (bitmask : 1 = admin)
	Rights uint `gorm:"default:0" json:"-"`

	// Options utilisateur (bitmask UserFlag)
	Flags uint `gorm:"default:0" json:"-"`

	// CGU acceptées
	TOS bool `gorm:"default:false" json:"-"`

	// Langue préférée
	Lang string `gorm:"size:2;default:'fr'" json:"lang"`

	// Dernière connexion
	LastLogin *time.Time `json:"ldate,omitempty"`

	// Vérification email (compte activé après confirmation par email)
	EmailVerifiedAt *time.Time `json:"-"`

	// Relations
	UserGroups []UserGroup `gorm:"foreignKey:UserID" json:"-"`
}

func (u *User) TableName() string { return "users" }

// IsAdmin retourne true si l'utilisateur est administrateur site-wide.
func (u *User) IsAdmin() bool {
	return u.Rights&1 != 0 || u.ID == 1
}

// HasFlag vérifie si un flag est activé.
func (u *User) HasFlag(f UserFlag) bool {
	return UserFlag(u.Flags)&f != 0
}

// SetFlag active un flag.
func (u *User) SetFlag(f UserFlag) {
	u.Flags = uint(UserFlag(u.Flags) | f)
}

// Name retourne "NOM Prénom".
func (u *User) Name() string {
	return strings.ToUpper(u.LastName) + " " + u.FirstName
}

// SetPassword encode le mot de passe avec MD5 + clé applicative.
// TODO: migrer vers bcrypt pour les nouveaux comptes.
func (u *User) SetPassword(plain, appKey string) {
	raw := appKey + strings.TrimSpace(plain)
	u.Pass = fmt.Sprintf("%x", md5.Sum([]byte(raw)))
}

// CheckPassword vérifie le mot de passe.
func (u *User) CheckPassword(plain, appKey string) bool {
	raw := appKey + strings.TrimSpace(plain)
	hash := fmt.Sprintf("%x", md5.Sum([]byte(raw)))
	return u.Pass == hash
}

// IsFullyRegistered retourne true si le compte est activé (mot de passe défini).
func (u *User) IsFullyRegistered() bool {
	return u.Pass != ""
}

// BeforeSave normalise les champs avant toute écriture en base.
func (u *User) BeforeSave(_ *gorm.DB) error {
	u.Email = strings.ToLower(strings.TrimSpace(u.Email))
	if u.Email2 != nil {
		v := strings.ToLower(strings.TrimSpace(*u.Email2))
		u.Email2 = &v
	}
	u.LastName = strings.ToUpper(strings.TrimSpace(u.LastName))
	if u.LastName2 != nil {
		v := strings.ToUpper(strings.TrimSpace(*u.LastName2))
		u.LastName2 = &v
	}
	return nil
}
