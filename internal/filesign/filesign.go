// Package filesign construit les adresses signées sous lesquelles les fichiers
// stockés en base sont servis.
//
// Extrait du paquet handler parce que le service d'envoi de courriers en a
// besoin lui aussi, et qu'il ne peut pas l'importer : c'est handler qui dépend
// de service, pas l'inverse. Recopier la signature des deux côtés l'aurait fait
// diverger au premier changement, et des images cesseraient de s'afficher sans
// que rien ne le signale.
package filesign

import (
	"crypto/md5"
	"fmt"
	"strings"
)

// Sign reproduit la logique Haxe d'origine : id + "_" + md5(id + clé).
func Sign(id uint, key string) string {
	raw := fmt.Sprintf("%d%s", id, key)
	return fmt.Sprintf("%d_%x", id, md5.Sum([]byte(raw)))
}

// URL : le chemin servi par /file/:sign, extension comprise. L'extension ne
// sert qu'aux clients qui devinent le type par le nom ; la route la retire.
func URL(id uint, key, name string) string {
	ext := "png"
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		ext = name[dot+1:]
	}
	return fmt.Sprintf("/file/%s.%s", Sign(id, key), ext)
}
