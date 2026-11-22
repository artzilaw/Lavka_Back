package authservice

import (
	"context"

	_ "lavka/internal/grpc/codec"
	"lavka/internal/domain/models"

	"google.golang.org/grpc"
)

const ServiceName = "lavka.auth.v1.AuthService"

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type RegisterResponse struct {
	UserID int64 `json:"user_id"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string      `json:"token"`
	User  models.User `json:"user"`
}

type ValidateTokenRequest struct {
	Token string `json:"token"`
}

type LogoutRequest struct {
	JTI     string `json:"jti"`
	ExpUnix int64  `json:"exp_unix"`
}

type IsTokenRevokedRequest struct {
	JTI string `json:"jti"`
}

type IsTokenRevokedResponse struct {
	Revoked bool `json:"revoked"`
}

type GetProfileRequest struct {
	UserID int64 `json:"user_id"`
}

type UpdateProfileRequest struct {
	UserID int64  `json:"user_id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
}

type ChangePasswordRequest struct {
	UserID          int64  `json:"user_id"`
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type UpdateAvatarRequest struct {
	UserID   int64  `json:"user_id"`
	FileData []byte `json:"file_data"`
	Filename string `json:"filename"`
}

type UserResponse struct {
	User models.User `json:"user"`
}

type EmptyResponse struct{}

type AuthServiceServer interface {
	Register(context.Context, *RegisterRequest) (*RegisterResponse, error)
	Login(context.Context, *LoginRequest) (*LoginResponse, error)
	ValidateTokenAndGetUser(context.Context, *ValidateTokenRequest) (*UserResponse, error)
	Logout(context.Context, *LogoutRequest) (*EmptyResponse, error)
	IsTokenRevoked(context.Context, *IsTokenRevokedRequest) (*IsTokenRevokedResponse, error)
	GetProfile(context.Context, *GetProfileRequest) (*UserResponse, error)
	UpdateProfile(context.Context, *UpdateProfileRequest) (*UserResponse, error)
	ChangePassword(context.Context, *ChangePasswordRequest) (*EmptyResponse, error)
	UpdateAvatar(context.Context, *UpdateAvatarRequest) (*UserResponse, error)
}

func RegisterAuthServiceServer(s grpc.ServiceRegistrar, srv AuthServiceServer) {
	s.RegisterService(&AuthServiceDesc, srv)
}

func unaryHandler[Req any, Resp any](
	fn func(ctx context.Context, req *Req) (*Resp, error),
	fullMethod string,
) grpc.MethodHandler {
	return func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
		in := new(Req)
		if err := dec(in); err != nil {
			return nil, err
		}
		if interceptor == nil {
			return fn(ctx, in)
		}
		info := &grpc.UnaryServerInfo{
			Server:     srv,
			FullMethod: fullMethod,
		}
		handler := func(ctx context.Context, req any) (any, error) {
			return fn(ctx, req.(*Req))
		}
		return interceptor(ctx, in, info, handler)
	}
}

var AuthServiceDesc = grpc.ServiceDesc{
	ServiceName: ServiceName,
	HandlerType: (*AuthServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Register",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				return unaryHandler(srv.(AuthServiceServer).Register, "/"+ServiceName+"/Register")(srv, ctx, dec, interceptor)
			},
		},
		{
			MethodName: "Login",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				return unaryHandler(srv.(AuthServiceServer).Login, "/"+ServiceName+"/Login")(srv, ctx, dec, interceptor)
			},
		},
		{
			MethodName: "ValidateTokenAndGetUser",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				return unaryHandler(srv.(AuthServiceServer).ValidateTokenAndGetUser, "/"+ServiceName+"/ValidateTokenAndGetUser")(srv, ctx, dec, interceptor)
			},
		},
		{
			MethodName: "Logout",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				return unaryHandler(srv.(AuthServiceServer).Logout, "/"+ServiceName+"/Logout")(srv, ctx, dec, interceptor)
			},
		},
		{
			MethodName: "IsTokenRevoked",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				return unaryHandler(srv.(AuthServiceServer).IsTokenRevoked, "/"+ServiceName+"/IsTokenRevoked")(srv, ctx, dec, interceptor)
			},
		},
		{
			MethodName: "GetProfile",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				return unaryHandler(srv.(AuthServiceServer).GetProfile, "/"+ServiceName+"/GetProfile")(srv, ctx, dec, interceptor)
			},
		},
		{
			MethodName: "UpdateProfile",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				return unaryHandler(srv.(AuthServiceServer).UpdateProfile, "/"+ServiceName+"/UpdateProfile")(srv, ctx, dec, interceptor)
			},
		},
		{
			MethodName: "ChangePassword",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				return unaryHandler(srv.(AuthServiceServer).ChangePassword, "/"+ServiceName+"/ChangePassword")(srv, ctx, dec, interceptor)
			},
		},
		{
			MethodName: "UpdateAvatar",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				return unaryHandler(srv.(AuthServiceServer).UpdateAvatar, "/"+ServiceName+"/UpdateAvatar")(srv, ctx, dec, interceptor)
			},
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proto/auth/auth.proto",
}
