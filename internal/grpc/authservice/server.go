package authservice

import (
	"bytes"
	"context"
	"errors"
	"time"

	"lavka/internal/services/auth"
	"lavka/internal/storage"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TokenChecker interface {
	Check(jti string) bool
}

type Server struct {
	authSvc *auth.Auth
	bl      TokenChecker
}

func NewServer(authSvc *auth.Auth, bl TokenChecker) *Server {
	return &Server{
		authSvc: authSvc,
		bl:      bl,
	}
}

func mapAuthError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, storage.ErrUserExists):
		return status.Error(codes.AlreadyExists, storage.ErrUserExists.Error())
	case errors.Is(err, storage.ErrUserNotFound):
		return status.Error(codes.NotFound, storage.ErrUserNotFound.Error())
	case errors.Is(err, auth.ErrInvalidCredentials):
		return status.Error(codes.Unauthenticated, auth.ErrInvalidCredentials.Error())
	case errors.Is(err, auth.ErrTokenRevoked):
		return status.Error(codes.Unauthenticated, auth.ErrTokenRevoked.Error())
	case errors.Is(err, auth.ErrInvalidCurrentPassword):
		return status.Error(codes.InvalidArgument, auth.ErrInvalidCurrentPassword.Error())
	case errors.Is(err, auth.ErrSamePassword):
		return status.Error(codes.InvalidArgument, auth.ErrSamePassword.Error())
	case errors.Is(err, auth.ErrUnsupportedFileType):
		return status.Error(codes.InvalidArgument, auth.ErrUnsupportedFileType.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}

func (s *Server) Register(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error) {
	uid, err := s.authSvc.RegisterNewUser(ctx, req.Email, req.Password, req.Name)
	if err != nil {
		return nil, mapAuthError(err)
	}
	return &RegisterResponse{UserID: uid}, nil
}

func (s *Server) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	token, user, err := s.authSvc.Login(ctx, req.Email, req.Password)
	if err != nil {
		return nil, mapAuthError(err)
	}
	return &LoginResponse{Token: token, User: user}, nil
}

func (s *Server) ValidateTokenAndGetUser(ctx context.Context, req *ValidateTokenRequest) (*UserResponse, error) {
	user, err := s.authSvc.ValidateTokenAndGetUser(ctx, req.Token)
	if err != nil {
		return nil, mapAuthError(err)
	}
	return &UserResponse{User: user}, nil
}

func (s *Server) Logout(ctx context.Context, req *LogoutRequest) (*EmptyResponse, error) {
	if err := s.authSvc.Logout(ctx, req.JTI, time.Unix(req.ExpUnix, 0)); err != nil {
		return nil, mapAuthError(err)
	}
	return &EmptyResponse{}, nil
}

func (s *Server) IsTokenRevoked(ctx context.Context, req *IsTokenRevokedRequest) (*IsTokenRevokedResponse, error) {
	revoked := false
	if s.bl != nil {
		revoked = s.bl.Check(req.JTI)
	}
	return &IsTokenRevokedResponse{Revoked: revoked}, nil
}

func (s *Server) GetProfile(ctx context.Context, req *GetProfileRequest) (*UserResponse, error) {
	user, err := s.authSvc.GetProfile(ctx, req.UserID)
	if err != nil {
		return nil, mapAuthError(err)
	}
	return &UserResponse{User: user}, nil
}

func (s *Server) UpdateProfile(ctx context.Context, req *UpdateProfileRequest) (*UserResponse, error) {
	user, err := s.authSvc.UpdateProfile(ctx, req.UserID, req.Name, req.Email)
	if err != nil {
		return nil, mapAuthError(err)
	}
	return &UserResponse{User: user}, nil
}

func (s *Server) ChangePassword(ctx context.Context, req *ChangePasswordRequest) (*EmptyResponse, error) {
	if err := s.authSvc.ChangePassword(ctx, req.UserID, req.CurrentPassword, req.NewPassword); err != nil {
		return nil, mapAuthError(err)
	}
	return &EmptyResponse{}, nil
}

func (s *Server) UpdateAvatar(ctx context.Context, req *UpdateAvatarRequest) (*UserResponse, error) {
	user, err := s.authSvc.UpdateAvatar(ctx, req.UserID, bytes.NewReader(req.FileData), req.Filename)
	if err != nil {
		return nil, mapAuthError(err)
	}
	return &UserResponse{User: user}, nil
}
