package authservice

import (
	"context"
	"io"
	"time"

	"lavka/internal/domain/models"
	"lavka/internal/grpc/codec"
	"lavka/internal/grpc/interceptors"
	"lavka/internal/services/auth"
	"lavka/internal/storage"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// Client реализует интерфейсы httpapp.Auth и middleware.TokenChecker поверх gRPC.
type Client struct {
	conn *grpc.ClientConn
}

func NewClient(target string) (*Client, error) {
	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.CallContentSubtype(codec.Name)),
	)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn}, nil
}

func NewClientWithConn(conn *grpc.ClientConn) *Client {
	return &Client{conn: conn}
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Client) invoke(ctx context.Context, method string, in, out any) error {
	ctx = interceptors.OutgoingRequestIDContext(ctx)
	return c.conn.Invoke(ctx, "/"+ServiceName+"/"+method, in, out, grpc.CallContentSubtype(codec.Name))
}

func unmapAuthError(err error) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return err
	}
	msg := st.Message()
	switch st.Code() {
	case codes.AlreadyExists:
		return storage.ErrUserExists
	case codes.NotFound:
		return storage.ErrUserNotFound
	case codes.Unauthenticated:
		if msg == auth.ErrTokenRevoked.Error() {
			return auth.ErrTokenRevoked
		}
		return auth.ErrInvalidCredentials
	case codes.InvalidArgument:
		switch msg {
		case auth.ErrInvalidCurrentPassword.Error():
			return auth.ErrInvalidCurrentPassword
		case auth.ErrSamePassword.Error():
			return auth.ErrSamePassword
		case auth.ErrUnsupportedFileType.Error():
			return auth.ErrUnsupportedFileType
		}
	}
	return err
}

func (c *Client) RegisterNewUser(ctx context.Context, email, password, name string) (int64, error) {
	var resp RegisterResponse
	err := c.invoke(ctx, "Register", &RegisterRequest{
		Email:    email,
		Password: password,
		Name:     name,
	}, &resp)
	if err != nil {
		return 0, unmapAuthError(err)
	}
	return resp.UserID, nil
}

func (c *Client) Login(ctx context.Context, email, password string) (string, models.User, error) {
	var resp LoginResponse
	err := c.invoke(ctx, "Login", &LoginRequest{
		Email:    email,
		Password: password,
	}, &resp)
	if err != nil {
		return "", models.User{}, unmapAuthError(err)
	}
	return resp.Token, resp.User, nil
}

func (c *Client) ValidateTokenAndGetUser(ctx context.Context, tokenString string) (models.User, error) {
	var resp UserResponse
	err := c.invoke(ctx, "ValidateTokenAndGetUser", &ValidateTokenRequest{Token: tokenString}, &resp)
	if err != nil {
		return models.User{}, unmapAuthError(err)
	}
	return resp.User, nil
}

func (c *Client) Logout(ctx context.Context, jti string, exp time.Time) error {
	var resp EmptyResponse
	err := c.invoke(ctx, "Logout", &LogoutRequest{JTI: jti, ExpUnix: exp.Unix()}, &resp)
	return unmapAuthError(err)
}

// Check реализует middleware.TokenChecker, спрашивая у auth-service по gRPC, не отозван ли jti.
func (c *Client) Check(jti string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var resp IsTokenRevokedResponse
	if err := c.invoke(ctx, "IsTokenRevoked", &IsTokenRevokedRequest{JTI: jti}, &resp); err != nil {
		return false
	}
	return resp.Revoked
}

func (c *Client) GetProfile(ctx context.Context, userID int64) (models.User, error) {
	var resp UserResponse
	err := c.invoke(ctx, "GetProfile", &GetProfileRequest{UserID: userID}, &resp)
	if err != nil {
		return models.User{}, unmapAuthError(err)
	}
	return resp.User, nil
}

func (c *Client) UpdateProfile(ctx context.Context, userID int64, name string, email string) (models.User, error) {
	var resp UserResponse
	err := c.invoke(ctx, "UpdateProfile", &UpdateProfileRequest{
		UserID: userID,
		Name:   name,
		Email:  email,
	}, &resp)
	if err != nil {
		return models.User{}, unmapAuthError(err)
	}
	return resp.User, nil
}

func (c *Client) ChangePassword(ctx context.Context, userID int64, currentPassword, newPassword string) error {
	var resp EmptyResponse
	err := c.invoke(ctx, "ChangePassword", &ChangePasswordRequest{
		UserID:          userID,
		CurrentPassword: currentPassword,
		NewPassword:     newPassword,
	}, &resp)
	return unmapAuthError(err)
}

func (c *Client) UpdateAvatar(ctx context.Context, userID int64, file io.ReadSeeker, filename string) (models.User, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return models.User{}, err
	}
	var resp UserResponse
	err = c.invoke(ctx, "UpdateAvatar", &UpdateAvatarRequest{
		UserID:   userID,
		FileData: data,
		Filename: filename,
	}, &resp)
	if err != nil {
		return models.User{}, unmapAuthError(err)
	}
	return resp.User, nil
}
