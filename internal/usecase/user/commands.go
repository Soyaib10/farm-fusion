package user

type UpsertUserCommand struct {
	Name  *string `json:"name,omitempty" validate:"omitempty,min=1"`
	Email *string `json:"email,omitempty" validate:"omitempty,email"`
}