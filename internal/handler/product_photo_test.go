package handler

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// pngHeader fabrique un en-tête PNG avec la palette voulue. paletteSize à zéro
// produit une image en couleurs vraies, sans bloc PLTE.
func pngHeader(paletteSize int) []byte {
	var b bytes.Buffer
	b.WriteString("\x89PNG\r\n\x1a\n")

	bloc := func(kind string, payload int) {
		var l [4]byte
		binary.BigEndian.PutUint32(l[:], uint32(payload))
		b.Write(l[:])
		b.WriteString(kind)
		b.Write(make([]byte, payload))
		b.Write([]byte{0, 0, 0, 0}) // CRC, ignoré à la lecture
	}
	bloc("IHDR", 13)
	if paletteSize > 0 {
		bloc("PLTE", paletteSize*3)
	}
	bloc("IDAT", 64)
	return b.Bytes()
}

// Un pictogramme tient en quelques teintes ; une photographie convertie en
// palette en utilise des dizaines. Le seuil sépare les deux familles.
func TestPictogramsAreToldApartFromPhotos(t *testing.T) {
	cases := []struct {
		nom      string
		couleurs int
		photo    bool
	}{
		{"dessin importé (9 teintes)", 9, false},
		{"illustration générique (16)", 16, false},
		{"juste sous le seuil", minPaletteColors - 1, false},
		{"au seuil", minPaletteColors, true},
		{"photo convertie (100)", 100, true},
		{"palette pleine (256)", 256, true},
		{"couleurs vraies, sans palette", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.nom, func(t *testing.T) {
			if got := looksLikePhoto(pngHeader(tc.couleurs)); got != tc.photo {
				t.Errorf("obtenu photo=%v, attendu %v", got, tc.photo)
			}
		})
	}
}

// Dans le doute, on garde : écarter à tort la photo d'un producteur est plus
// dommageable que laisser passer un dessin.
func TestUnknownFormatsAreKept(t *testing.T) {
	for nom, donnees := range map[string][]byte{
		"JPEG":            {0xFF, 0xD8, 0xFF, 0xE0, 0, 16, 'J', 'F'},
		"vide":            nil,
		"en-tête tronqué": []byte("\x89PNG\r\n\x1a\n\x00\x00"),
	} {
		if !looksLikePhoto(donnees) {
			t.Errorf("%s : devrait être conservé faute de pouvoir juger", nom)
		}
	}
}

// Une étiquette est faite d'aplats : quelques teintes couvrent l'essentiel de
// la surface. Une photographie, même sur fond clair, garde des dégradés.
func TestFlatArtworkIsNotAPhotograph(t *testing.T) {
	// Étiquette : un fond blanc et deux encres.
	etiquette := image.NewRGBA(image.Rect(0, 0, 60, 60))
	for y := 0; y < 60; y++ {
		for x := 0; x < 60; x++ {
			c := color.RGBA{255, 255, 255, 255}
			if y > 20 && y < 30 {
				c = color.RGBA{200, 30, 30, 255}
			}
			etiquette.Set(x, y, c)
		}
	}
	if IsPhotograph(encodePNG(t, etiquette)) {
		t.Error("une image d'aplats ne devrait pas passer pour une photographie")
	}

	// Photographie simulée : chaque pixel diffère de ses voisins.
	photo := image.NewRGBA(image.Rect(0, 0, 60, 60))
	for y := 0; y < 60; y++ {
		for x := 0; x < 60; x++ {
			photo.Set(x, y, color.RGBA{uint8(x*4 + y), uint8(y*3 + x*2), uint8(x + y*5), 255})
		}
	}
	if !IsPhotograph(encodePNG(t, photo)) {
		t.Error("une image en dégradés devrait être tenue pour une photographie")
	}
}

// Dans le doute, on garde.
func TestUnreadableImagesAreKept(t *testing.T) {
	if !IsPhotograph([]byte("ceci n'est pas une image")) {
		t.Error("une image illisible devrait être conservée")
	}
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatalf("encodage : %v", err)
	}
	return b.Bytes()
}
