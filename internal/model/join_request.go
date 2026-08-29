package model

import "time"

// JoinRequestStatus : où en est une demande d'adhésion.
type JoinRequestStatus string

const (
	JoinRequestPending  JoinRequestStatus = "Pending"
	JoinRequestAccepted JoinRequestStatus = "Accepted"
	JoinRequestRefused  JoinRequestStatus = "Refused"
)

// GroupJoinRequest : demande d'adhésion d'un compte à un groupe, déposée à
// l'inscription et tranchée par un gestionnaire des membres.
//
// C'est elle que le mode d'inscription RegOptionWaitingList désignait sans que
// rien ne l'implémente : son acceptation crée l'UserGroup.
//
// Une seule demande par couple (compte, groupe) : un candidat refusé qui
// retente réécrit la sienne plutôt que d'en empiler une seconde, faute de quoi
// l'écran des gestionnaires afficherait le même nom autant de fois qu'il a
// insisté.
type GroupJoinRequest struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `json:"cdate"`
	UpdatedAt time.Time `json:"-"`

	UserID  uint  `gorm:"not null;uniqueIndex:idx_join_user_group" json:"userId"`
	User    User  `gorm:"foreignKey:UserID" json:"-"`
	GroupID uint  `gorm:"not null;uniqueIndex:idx_join_user_group" json:"groupId"`
	Group   Group `gorm:"foreignKey:GroupID" json:"-"`

	Status JoinRequestStatus `gorm:"size:16;not null;default:'Pending';index" json:"status"`

	// Mot laissé par le candidat au moment de sa demande. Facultatif : c'est
	// souvent là qu'il dit qui l'a orienté vers le groupe, et le gestionnaire
	// n'a rien d'autre pour reconnaître un inconnu.
	Message *string `gorm:"type:text" json:"message,omitempty"`

	// Trace de la décision. Le refus ne supprime pas la ligne : sans elle, une
	// demande écartée réapparaîtrait à la première réinscription sans que
	// personne ne sache qu'elle a déjà été examinée.
	DecidedAt   *time.Time `json:"decidedAt,omitempty"`
	DecidedByID *uint      `json:"-"`
	DecidedBy   *User      `gorm:"foreignKey:DecidedByID" json:"-"`
}

func (r *GroupJoinRequest) TableName() string { return "group_join_requests" }

// IsPending : la demande attend encore une décision.
func (r *GroupJoinRequest) IsPending() bool { return r.Status == JoinRequestPending }
