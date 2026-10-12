package domain

// ActualizarPerfilDTO define los campos opcionales para la actualización parcial del perfil de usuario
type ActualizarPerfilDTO struct {
	NombreCompleto *string `json:"nombre_completo,omitempty"`
	Correo         *string `json:"correo,omitempty"`
	Telefono       *string `json:"telefono,omitempty"`
	Direccion      *string `json:"direccion,omitempty"`
	Password       *string `json:"password,omitempty"`
}

// PerfilUsuarioDTO representa la respuesta con los datos del usuario tras la actualización o consulta
type PerfilUsuarioDTO struct {
	ID             string  `json:"id"`
	Correo         string  `json:"correo"`
	NombreCompleto string  `json:"nombre_completo"`
	Telefono       *string `json:"telefono,omitempty"`
	Direccion      *string `json:"direccion,omitempty"`
	Rol            string  `json:"rol"`
	EstaActivo     bool    `json:"esta_activo"`
	Mensaje        string  `json:"mensaje,omitempty"`
}
