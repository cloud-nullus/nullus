package usecase

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/cloud-nullus/draft/internal/admin/port"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
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

func (uc *UserUseCase) SearchByEmail(ctx context.Context, email string) (*domain.User, error) {
	user, err := uc.userRepo.SearchByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("searching user by email: %w", err)
	}
	return user, nil
}

func (uc *UserUseCase) InviteMember(ctx context.Context, orgID, email, name string, role domain.Role) (*domain.User, error) {
	if role == "" {
		role = domain.RoleDeveloper
	}

	existingUser, err := uc.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("checking user by email: %w", err)
	}
	if existingUser != nil {
		isMember, err := uc.userRepo.IsMember(ctx, orgID, existingUser.ID)
		if err != nil {
			return nil, fmt.Errorf("checking organization membership: %w", err)
		}
		if isMember {
			return nil, &shareddomain.AppError{
				Code:       "USER_ALREADY_MEMBER",
				HTTPStatus: http.StatusConflict,
				Message:    "User is already a member",
				Detail:     fmt.Sprintf("user with email %q already belongs to organization %q", email, orgID),
				Retryable:  false,
			}
		}

		if err := uc.userRepo.AddMember(ctx, orgID, existingUser.ID, role); err != nil {
			return nil, fmt.Errorf("adding existing member: %w", err)
		}

		existingUser.OrgID = orgID
		existingUser.Role = role
		existingUser.IsActive = true
		return existingUser, nil
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

	if err := uc.userRepo.AddMember(ctx, orgID, user.ID, role); err != nil {
		return nil, fmt.Errorf("adding invited member: %w", err)
	}

	return user, nil
}

func (uc *UserUseCase) UpdateRole(ctx context.Context, userID string, newRole domain.Role) error {
	_, err := uc.UpdateMember(ctx, userID, UpdateMemberInput{Role: &newRole})
	return err
}

type UpdateMemberInput struct {
	OrgID string
	Name  *string
	Email *string
	Role  *domain.Role
}

func (uc *UserUseCase) UpdateMember(ctx context.Context, userID string, input UpdateMemberInput) (*domain.User, error) {
	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("getting user: %w", err)
	}
	if user == nil {
		return nil, &shareddomain.AppError{
			Code:       "USER_NOT_FOUND",
			HTTPStatus: http.StatusNotFound,
			Message:    "User not found",
			Detail:     fmt.Sprintf("user with id %q does not exist", userID),
			Retryable:  false,
		}
	}

	if input.Name != nil {
		trimmed := strings.TrimSpace(*input.Name)
		if trimmed == "" {
			return nil, &shareddomain.AppError{
				Code:       "USER_NAME_REQUIRED",
				HTTPStatus: http.StatusBadRequest,
				Message:    "Name is required",
				Detail:     "name cannot be empty",
				Retryable:  false,
			}
		}
		user.Name = trimmed
	}

	if input.Email != nil {
		trimmed := strings.TrimSpace(*input.Email)
		if trimmed == "" {
			return nil, &shareddomain.AppError{
				Code:       "USER_EMAIL_REQUIRED",
				HTTPStatus: http.StatusBadRequest,
				Message:    "Email is required",
				Detail:     "email cannot be empty",
				Retryable:  false,
			}
		}
		user.Email = trimmed
	}

	if input.Role != nil {
		user.Role = *input.Role
	}
	if input.OrgID != "" {
		user.OrgID = input.OrgID
	}

	user.UpdatedAt = time.Now().UTC()

	if err := uc.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("updating user: %w", err)
	}

	return user, nil
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
