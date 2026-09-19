package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"sync"
)

// UNE ADRESSE NEUVE QUAND LA FEUILLE CHANGE, ET SEULEMENT ALORS.
//
// Les gabarits demandaient « /css/alterconso.css », sans rien qui distingue
// une version de la suivante. Un navigateur qui tenait l'ancienne en cache la
// resservait donc avec le HTML neuf — et les deux ne s'accordent pas : une
// classe posee par le gabarit d'aujourd'hui n'existe pas dans la feuille
// d'hier, si bien que ce qu'elle devait cacher s'affiche. On a vu les trois
// lignes d'informations ET le bouton cense les remplacer, sur le meme ecran.
//
// L'empreinte du CONTENU, et non la version du binaire : elle ne change que
// lorsque la feuille change vraiment, la ou un numero de commit forcerait un
// telechargement a chaque livraison. Elle vaut aussi en developpement, ou il
// n'y a pas de version a injecter.
//
// Calculee une fois, a la premiere page servie : les fichiers ne bougent plus
// une fois l'image construite.
var (
	assetVersionOnce  sync.Once
	assetVersionValue string
)

// assetFiles : les feuilles dont le contenu determine l'empreinte. Les
// bibliotheques figees — bootstrap, la police — n'y figurent pas : leur nom ne
// change jamais parce que leur contenu ne change pas non plus.
var assetFiles = []string{
	"www/css/alterconso.css",
	"www/css/style.css",
}

// assetVersion rend une empreinte courte des feuilles de style, a poser en
// parametre d'URL. « dev » si rien n'est lisible — mieux vaut un cache
// discutable qu'une page sans style.
func assetVersion() string {
	assetVersionOnce.Do(func() { assetVersionValue = empreinteDes(assetFiles) })
	return assetVersionValue
}

// empreinteDes rend une empreinte courte du contenu des fichiers donnes, en
// acceptant leurs formes compressees. « dev » si aucun n'est lisible — mieux
// vaut un cache discutable qu'une page sans style.
func empreinteDes(fichiers []string) string {
	somme := sha256.New()
	lu := false
	for _, f := range fichiers {
		// LE FICHIER BRUT N'EXISTE PAS DANS L'IMAGE. Le build precompresse les
		// feuilles — « StaticPrecompressed » ne sert que du .br ou du .gz — et
		// « alterconso.css » tout court n'y figure pas. Faute de le trouver,
		// l'empreinte retombait sur « dev » : figee, donc inutile, ce qui
		// privait de tout effet le versionnage qu'elle devait porter.
		//
		// L'ordre compte peu : c'est le meme contenu sous trois formes, et
		// n'importe laquelle change des que la feuille change. On prend la
		// premiere qui repond, en developpement comme en production.
		for _, essai := range []string{f, f + ".br", f + ".gz"} {
			contenu, err := os.ReadFile(essai)
			if err != nil {
				continue
			}
			somme.Write(contenu)
			lu = true
			break
		}
	}
	if !lu {
		return "dev"
	}
	return hex.EncodeToString(somme.Sum(nil))[:10]
}
