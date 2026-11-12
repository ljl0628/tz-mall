package service

import (
	"context"
	"strings"
	"time"

	"fawn-mall/internal/model"
	"fawn-mall/internal/repository"
	"fawn-mall/pkg/jwtx"
	"fawn-mall/pkg/password"
	"fawn-mall/pkg/resp"
)

// AuthService 用户认证业务。
type AuthService struct {
	users *repository.UserRepo
	black *repository.Blacklist
	jwt   *jwtx.Manager
}

// NewAuthService 构造。
func NewAuthService(users *repository.UserRepo, black *repository.Blacklist, jwt *jwtx.Manager) *AuthService {
	return &AuthService{users: users, black: black, jwt: jwt}
}

// RegisterReq 注册请求。
type RegisterReq struct {
	Username string `json:"username" binding:"required,min=3,max=32,alphanum"`
	Password string `json:"password" binding:"required,min=6,max=64"`
	Nickname string `json:"nickname" binding:"max=32"`
}

// LoginReq 登录请求。
type LoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResp 登录响应。
type LoginResp struct {
	Token string     `json:"token"`
	User  model.User `json:"user"`
}

// Register 注册新用户(默认 user 角色)。
func (s *AuthService) Register(ctx context.Context, req RegisterReq) (*LoginResp, error) {
	if exist, err := s.users.GetByUsername(req.Username); err != nil {
		return nil, err
	} else if exist != nil {
		return nil, resp.ErrUserExists
	}
	nickname := strings.TrimSpace(req.Nickname)
	if nickname == "" {
		nickname = req.Username
	}
	hash, err := password.Hash(req.Password)
	if err != nil {
		return nil, err
	}
	u := &model.User{
		Username:     req.Username,
		PasswordHash: hash,
		Nickname:     nickname,
		Role:         model.RoleUser,
	}
	if err := s.users.Create(u); err != nil {
		return nil, err
	}
	token, err := s.jwt.Generate(u.ID, u.Role, u.Username)
	if err != nil {
		return nil, err
	}
	return &LoginResp{Token: token, User: *u}, nil
}

// Login 用户名密码登录。
func (s *AuthService) Login(ctx context.Context, req LoginReq) (*LoginResp, error) {
	u, err := s.users.GetByUsername(req.Username)
	if err != nil {
		return nil, err
	}
	if u == nil || !password.Verify(u.PasswordHash, req.Password) {
		return nil, resp.ErrLoginFailed
	}
	token, err := s.jwt.Generate(u.ID, u.Role, u.Username)
	if err != nil {
		return nil, err
	}
	return &LoginResp{Token: token, User: *u}, nil
}

// Logout 登出:token 摘要写入 Redis 黑名单,剩余有效期内拒绝该 token。
func (s *AuthService) Logout(ctx context.Context, token string, expireAt time.Time) error {
	ttl := time.Until(expireAt)
	if err := s.black.Add(ctx, token, ttl); err != nil {
		return err
	}
	return nil
}

// Me 查询当前用户信息。
func (s *AuthService) Me(ctx context.Context, userID uint64) (*model.User, error) {
	u, err := s.users.GetByID(userID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, resp.NewBiz(resp.CodeUnauthorized, "用户不存在")
	}
	return u, nil
}
