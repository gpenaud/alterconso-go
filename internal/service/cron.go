package service

import (
	"fmt"
	"log"
	"time"

	"github.com/gpenaud/alterconso/internal/config"
	"github.com/gpenaud/alterconso/internal/model"
	"github.com/gpenaud/alterconso/pkg/mailer"
	"gorm.io/gorm"
)

// CronService gère les tâches planifiées de l'application.
type CronService struct {
	db     *gorm.DB
	mailer *mailer.Mailer
	cfg    *config.Config
	dryRun bool
}

func NewCronService(db *gorm.DB, m *mailer.Mailer, cfg *config.Config) *CronService {
	return &CronService{db: db, mailer: m, cfg: cfg}
}

// SetDryRun active le mode dry-run : affiche les emails sans les envoyer.
func (s *CronService) SetDryRun(v bool) { s.dryRun = v }

// Start lance la boucle de tâches planifiées dans une goroutine.
// tick : intervalle entre chaque exécution (ex: time.Hour en prod, time.Minute en test).
func (s *CronService) Start(tick time.Duration) {
	go func() {
		for range time.Tick(tick) {
			s.RunAll()
		}
	}()
	log.Printf("[CRON] started (tick=%s)", tick)
}

// RunAll exécute toutes les tâches cron.
func (s *CronService) RunAll() {
	s.NotifyOrderOpenings()
	s.NotifyOrderClosingSoon()
	s.NotifyUpcomingDistributions()
	s.AutoValidatePastDistributions()
}

// NotifyUpcomingDistributions envoie des emails de rappel 24h et 4h avant chaque distribution.
func (s *CronService) NotifyUpcomingDistributions() {
	now := time.Now()

	windows := []struct {
		hours    int
		flagBit  model.UserFlag
		label    string
	}{
		{24, model.UserFlagEmailNotif24h, "24h"},
		{4, model.UserFlagEmailNotif4h, "4h"},
	}

	for _, w := range windows {
		from := now.Add(time.Duration(w.hours) * time.Hour)
		to := from.Add(time.Hour) // fenêtre d'1h pour éviter les doublons

		var mds []model.MultiDistrib
		s.db.Preload("Distributions.Catalog.Group").
			Where("distrib_start_date >= ? AND distrib_start_date < ? AND NOT validated", from, to).
			Find(&mds)

		for _, md := range mds {
			s.notifyMembersForDistrib(md, w.flagBit, w.label)
		}
	}
}

func (s *CronService) notifyMembersForDistrib(md model.MultiDistrib, flag model.UserFlag, label string) {
	if len(md.Distributions) == 0 {
		return
	}

	groupID := md.Distributions[0].Catalog.GroupID

	// Récupérer les membres du groupe qui ont activé la notification
	var members []model.UserGroup
	s.db.Preload("User").
		Where("group_id = ?", groupID).
		Find(&members)

	for _, ug := range members {
		if ug.User.HasFlag(flag) {
			s.sendDistribReminder(ug.User, md, label)
		}
	}
}

func (s *CronService) sendDistribReminder(u model.User, md model.MultiDistrib, label string) {
	subject := fmt.Sprintf("Rappel : distribution dans %s", label)
	html := fmt.Sprintf(`<p>Bonjour %s,</p>
<p>Une distribution est prévue dans <strong>%s</strong> le <strong>%s</strong>.</p>
<p>Pensez à vérifier vos commandes !</p>`,
		u.FirstName,
		label,
		md.DistribStartDate.Format("02/01/2006 à 15:04"),
	)
	if err := s.mailer.QuickMail(u.Email, subject, html); err != nil {
		log.Printf("[CRON] email reminder failed for user %d: %v", u.ID, err)
	}
}

// AutoValidatePastDistributions valide automatiquement les distributions passées.
// Une distribution est considérée passée si DistribEndDate < maintenant et n'est pas encore validée.
func (s *CronService) AutoValidatePastDistributions() {
	now := time.Now()

	var mds []model.MultiDistrib
	s.db.Where("distrib_end_date < ? AND NOT validated", now).Find(&mds)

	ps := NewPaymentService(s.db)
	for _, md := range mds {
		if err := ps.ValidateDistribution(md.ID); err != nil {
			log.Printf("[CRON] auto-validate distribution %d failed: %v", md.ID, err)
		} else {
			log.Printf("[CRON] auto-validated distribution %d", md.ID)
		}
	}
}
