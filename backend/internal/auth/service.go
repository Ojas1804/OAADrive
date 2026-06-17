package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"family-cloud/internal/models"
	"family-cloud/pkg/config"
)

const maxFailedLogins = 5
const lockDuration = 15 * time.Minute
const inviteTTL = 72 * time.Hour

type Service struct {
	repo *Repository
	cfg  *config.Config
}

func NewService(repo *Repository, cfg *config.Config) *Service {
	return &Service{repo: repo, cfg: cfg}
}

type RegisterAdminInput struct {
	Email    string
	Password string
}

type LoginInput struct {
	Email    string
	Password string
}

type InviteInput struct {
	Email string
	Role  models.Role
}

type AcceptInviteInput struct {
	Token    string
	Password string
}

func (s *Service) RegisterAdmin(ctx context.Context, in RegisterAdminInput) (*models.User, *TokenPair, error) {
	exists, err := s.repo.AdminExists(ctx)
	if err != nil {
		return nil, nil, err
	}
	if exists {
		return nil, nil, ErrAdminExists
	}

	hash, err := HashPassword(in.Password)
	if err != nil {
		return nil, nil, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.repo.CreateUser(ctx, in.Email, hash, models.RoleAdmin)
	if err != nil {
		return nil, nil, err
	}

	tokens, err := s.issueTokenPair(ctx, user)
	if err != nil {
		return nil, nil, err
	}
	return user, tokens, nil
}

func (s *Service) Login(ctx context.Context, in LoginInput) (*models.User, *TokenPair, error) {
	user, err := s.repo.GetUserByEmail(ctx, in.Email)
	if err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	if !user.IsActive {
		return nil, nil, ErrAccountInactive
	}

	if user.LockedUntil != nil && time.Now().Before(*user.LockedUntil) {
		return nil, nil, ErrAccountLocked
	}

	ok, err := VerifyPassword(in.Password, user.PasswordHash)
	if err != nil {
		return nil, nil, fmt.Errorf("verify password: %w", err)
	}
	if !ok {
		_ = s.repo.IncrementFailedLogins(ctx, user.ID)
		if user.FailedLogins+1 >= maxFailedLogins {
			until := time.Now().Add(lockDuration)
			_ = s.repo.LockUser(ctx, user.ID, until)
		}
		return nil, nil, ErrInvalidCredentials
	}

	_ = s.repo.ResetFailedLogins(ctx, user.ID)

	tokens, err := s.issueTokenPair(ctx, user)
	if err != nil {
		return nil, nil, err
	}
	return user, tokens, nil
}

func (s *Service) RefreshTokens(ctx context.Context, plainToken string) (*models.User, *TokenPair, error) {
	tokenHash := HashToken(plainToken)

	rt, err := s.repo.GetRefreshToken(ctx, tokenHash)
	if err != nil {
		return nil, nil, ErrInvalidToken
	}

	if err := s.repo.RevokeRefreshToken(ctx, tokenHash); err != nil {
		return nil, nil, err
	}

	user, err := s.repo.GetUserByID(ctx, rt.UserID)
	if err != nil {
		return nil, nil, err
	}

	if !user.IsActive {
		return nil, nil, ErrAccountInactive
	}

	tokens, err := s.issueTokenPair(ctx, user)
	if err != nil {
		return nil, nil, err
	}
	return user, tokens, nil
}

func (s *Service) Logout(ctx context.Context, plainToken string) error {
	tokenHash := HashToken(plainToken)
	return s.repo.RevokeRefreshToken(ctx, tokenHash)
}

func (s *Service) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	return s.repo.RevokeAllUserTokens(ctx, userID)
}

func (s *Service) InviteMember(ctx context.Context, adminID uuid.UUID, in InviteInput) (*models.Invitation, string, error) {
	plain, hashed, err := GenerateRefreshToken()
	if err != nil {
		return nil, "", fmt.Errorf("generate invite token: %w", err)
	}

	expiresAt := time.Now().Add(inviteTTL)
	inv, err := s.repo.CreateInvitation(ctx, in.Email, in.Role, hashed, adminID, expiresAt)
	if err != nil {
		return nil, "", err
	}
	return inv, plain, nil
}

func (s *Service) AcceptInvite(ctx context.Context, in AcceptInviteInput) (*models.User, *TokenPair, error) {
	tokenHash := HashToken(in.Token)

	inv, err := s.repo.GetInvitationByToken(ctx, tokenHash)
	if err != nil {
		return nil, nil, ErrInvalidToken
	}

	hash, err := HashPassword(in.Password)
	if err != nil {
		return nil, nil, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.repo.CreateUser(ctx, inv.Email, hash, inv.Role)
	if err != nil {
		return nil, nil, err
	}

	if err := s.repo.AcceptInvitation(ctx, tokenHash); err != nil {
		return nil, nil, err
	}

	tokens, err := s.issueTokenPair(ctx, user)
	if err != nil {
		return nil, nil, err
	}
	return user, tokens, nil
}

func (s *Service) issueTokenPair(ctx context.Context, user *models.User) (*TokenPair, error) {
	accessToken, expiresAt, err := GenerateAccessToken(
		user.ID, user.Email, string(user.Role),
		s.cfg.JWTSecret, s.cfg.AccessTokenTTL,
	)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	plain, hashed, err := GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	rtExpiry := time.Now().Add(time.Duration(s.cfg.RefreshTokenTTL) * 24 * time.Hour)
	if _, err := s.repo.CreateRefreshToken(ctx, user.ID, hashed, rtExpiry); err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: plain,
		ExpiresAt:    expiresAt,
	}, nil
}
