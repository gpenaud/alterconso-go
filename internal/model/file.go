package model

import "time"

// File : fichier binaire stocké en base (images produits, vendeurs, etc.)
type File struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `gorm:"column:cdate;autoCreateTime" json:"cdate"`
	Name      string    `gorm:"size:255" json:"name"`
	Data      []byte    `gorm:"type:mediumblob" json:"-"`
}

func (f *File) TableName() string { return "File" }

func (f *File) Extension() string {
	if f.Name == "" {
		return "png"
	}
	parts := splitExt(f.Name)
	if len(parts) < 2 {
		return "png"
	}
	return parts[1]
}

func splitExt(name string) []string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			return []string{name[:i], name[i+1:]}
		}
	}
	return []string{name}
}
