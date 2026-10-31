package domain

import (
	"time"

	"github.com/google/uuid"
)

// AuditoriaAdminUsuario registra quién cambió el estado o rol de una cuenta.
// Se escribe dentro de la misma transacción que la actualización.
type AuditoriaAdminUsuario struct {
	ID         int64                  `gorm:"primaryKey" json:"id"`
	ActorID    uuid.UUID              `gorm:"type:uuid;not null" json:"actor_id"`
	ObjetivoID uuid.UUID              `gorm:"type:uuid;not null" json:"objetivo_id"`
	Cambios    map[string]interface{} `gorm:"serializer:json;type:jsonb;not null" json:"cambios"`
	CreadoEl   time.Time              `gorm:"default:now();not null" json:"creado_el"`
}

func (AuditoriaAdminUsuario) TableName() string {
	return "api.auditoria_admin_usuarios"
}
