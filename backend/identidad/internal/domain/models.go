package domain

import (
	"time"

	"github.com/google/uuid"
)

// Usuario representa la entidad de usuarios centralizados en api.usuarios
type Usuario struct {
	ID                uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	GoogleID          *string    `gorm:"type:varchar(255);unique" json:"google_id,omitempty"`
	Correo            string     `gorm:"type:varchar(255);unique;not null" json:"correo"`
	PasswordHash      *string    `gorm:"type:varchar(255)" json:"-"`
	NombreCompleto    string     `gorm:"type:varchar(255);not null" json:"nombre_completo"`
	Telefono          *string    `gorm:"type:varchar(50)" json:"telefono,omitempty"`
	Direccion         *string    `gorm:"type:varchar(255)" json:"direccion,omitempty"`
	URLAvatar         *string    `gorm:"type:varchar(500)" json:"url_avatar,omitempty"`
	Rol               string     `gorm:"type:varchar(30);default:'registrado';not null" json:"rol"`
	CuotaTokensIA     int        `gorm:"default:1000;not null" json:"cuota_tokens_ia"`
	EstaActivo        bool       `gorm:"default:true;not null" json:"esta_activo"`
	TokenVerificacion *uuid.UUID `gorm:"type:uuid" json:"token_verificacion,omitempty"`
	CreadoEl          time.Time  `gorm:"default:now();not null" json:"creado_el"`
	ActualizadoEl     time.Time  `gorm:"default:now();not null" json:"actualizado_el"`
}

func (Usuario) TableName() string {
	return "api.usuarios"
}
