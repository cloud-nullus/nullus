package usecase

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/cloud-nullus/draft/internal/admin/port"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	"github.com/google/uuid"
)

type UserUseCase struct {
	userRepo port.UserRepository
}

func NewUserUseCase(userRepo port.UserRepository) *UserUseCase {
	return &UserUseCase{userRepo: userRepo}
}

func (uc *UserUseCase) ListMembers(ctx context.Context, orgID string) ([]*domain.User, error) {
	users, err := uc.userRepo.ListByOrg(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("listing members: %w", err)
	}
	return users, nil
}

func (uc *UserUseCase) InviteMember(ctx context.Context, orgID, email, name string, role domain.Role) (*domain.User, error) {
	if role == "" {
		role = domain.RoleDeveloper
	}

	now := time.Now().UTC()
	user := &domain.User{
		ID:        uuid.NewString(),
		Email:     email,
		Name:      name,
		Role:      role,
		OrgID:     orgID,
		IsActive:  false,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := uc.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("inviting member: %w", err)
	}

	return user, nil
}

func (uc *UserUseCase) UpdateRole(ctx context.Context, userID string, newRole domain.Role) error {
	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("getting user: %w", err)
	}
	if user == nil {
		return &shareddomain.AppError{
			Code:       "USER_NOT_FOUND",
			HTTPStatus: http.StatusNotFound,
			Message:    "User not found",
			Detail:     fmt.Sprintf("user with id %q does not exist", userID),
			Retryable:  false,
		}
	}

	user.Role = newRole
	user.UpdatedAt = time.Now().UTC()

	if err := uc.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("updating user role: %w", err)
	}

	return nil
}

func (uc *UserUseCase) DeactivateUser(ctx context.Context, userID string) error {
	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("getting user: %w", err)
	}
	if user == nil {
		return &shareddomain.AppError{
			Code:       "USER_NOT_FOUND",
			HTTPStatus: http.StatusNotFound,
			Message:    "User not found",
			Detail:     fmt.Sprintf("user with id %q does not exist", userID),
			Retryable:  false,
		}
	}

	user.IsActive = false
	user.UpdatedAt = time.Now().UTC()

	if err := uc.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("deactivating user: %w", err)
	}

	return nil
}

func (uc *UserUseCase) RemoveMember(ctx context.Context, userID string) error {
	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("getting user: %w", err)
	}
	if user == nil {
		return &shareddomain.AppError{
			Code:       "USER_NOT_FOUND",
			HTTPStatus: http.StatusNotFound,
			Message:    "User not found",
			Detail:     fmt.Sprintf("user with id %q does not exist", userID),
			Retryable:  false,
		}
	}

	if err := uc.userRepo.Delete(ctx, userID); err != nil {
		return fmt.Errorf("removing member: %w", err)
	}

	return nil
}
