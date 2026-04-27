package handler

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// BrevoQuota représente le quota d'envoi du jour côté Brevo.
type BrevoQuota struct {
	DailyLimit int
	Remaining  int
	Refreshed  time.Time
	Error      string
}

type brevoAccountResp struct {
	Plan []struct {
		Type        string `json:"type"`
		CreditsType string `json:"creditsType"`
		Credits     int    `json:"credits"`
	} `json:"plan"`
}

var (
	brevoCache    BrevoQuota
	brevoCacheMu  sync.Mutex
	brevoCacheTTL = 5 * time.Minute
)

// FetchBrevoQuota interroge l'API Brevo et met en cache le résultat 5 minutes.
// Si BrevoAPIKey est vide, retourne un quota vide avec une erreur.
func FetchBrevoQuota(apiKey string) BrevoQuota {
	brevoCacheMu.Lock()
	defer brevoCacheMu.Unlock()

	if apiKey == "" {
		return BrevoQuota{Error: "Clé API Brevo non configurée."}
	}
	if !brevoCache.Refreshed.IsZero() && time.Since(brevoCache.Refreshed) < brevoCacheTTL && brevoCache.Error == "" {
		return brevoCache
	}

	req, err := http.NewRequest("GET", "https://api.brevo.com/v3/account", nil)
	if err != nil {
		return BrevoQuota{Error: err.Error()}
	}
	req.Header.Set("api-key", apiKey)
	req.Header.Set("accept", "application/json")

	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return BrevoQuota{Error: "Brevo injoignable : " + err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return BrevoQuota{Error: "Brevo a répondu " + resp.Status}
	}

	var data brevoAccountResp
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return BrevoQuota{Error: "Réponse Brevo invalide."}
	}
	for _, p := range data.Plan {
		if p.CreditsType == "sendLimit" {
			q := BrevoQuota{
				DailyLimit: 300,
				Remaining:  p.Credits,
				Refreshed:  time.Now(),
			}
			brevoCache = q
			return q
		}
	}
	return BrevoQuota{Error: "Quota d'envoi introuvable dans la réponse Brevo."}
}
